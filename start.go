package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"log"
	"os"
	"regexp"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
)

type Configuration struct {
	Debug     bool   `json:"debug"`
	UserAgent string `json:"userAgent"`
	Request   struct {
		TimeoutInMs     int    `json:"timeoutInMs"`
		DomainGlob      string `json:"domainGlob"`
		Parallelism     int    `json:"parellelism"`
		DelayInMs       int    `json:"delayInMs"`
		RandomDelayInMs int    `json:"randomDelayInMs"`
	} `json:"request"`
	Input struct {
		StartUrl             string   `json:"startUrl"`
		UrlFilters           []string `json:"urlFilters"`
		DisallowedUrlFilters []string `json:"disallowedUrlFilters"`
	} `json:"input"`
	Output struct {
		Filename string `json:"filename"`
	}
	Html struct {
		Selectors map[string]string
	}
	Pdf struct {
		Enabled   bool `json:"Enabled"`
		Selectors map[string]string
	}
}

type HtmlSelectorTemplateVars struct {
	Request  colly.Request
	Response colly.Response
}

type PdfSelectorTemplateVars struct {
	Request     colly.Request
	TextContent string
	Meta        map[string]string
}

func ChildTexts(el *colly.HTMLElement, goquerySelector string) []string {
	var res []string

	// we special-case commas in selectors to allow content to be returned in the order specified, rather than document order
	selectors := strings.Split(goquerySelector, ",")

	for _, sel := range selectors {

		el.DOM.Find(sel).Each(func(_ int, s *goquery.Selection) {

			withoutNewlines := strings.Replace(s.Text(), "\n", "", -1)

			doubleWhiteSpaceRegex := regexp.MustCompile(`[\s\p{Zs}]{2,}`)
			withoutExtraSpaces := doubleWhiteSpaceRegex.ReplaceAllString(withoutNewlines, " ")

			res = append(res, withoutExtraSpaces)
		})

	}
	return res
}

func loadConfiguration() Configuration {
	// Use environment variable for configuration if available.
	// This helps support passing it in when using Docker.
	configuration := Configuration{}
	val, found := os.LookupEnv("CONFIG")

	if found {
		err := json.Unmarshal([]byte(val), &configuration)
		if err != nil {
			log.Fatal(err)
		}
	} else {

		file, _ := os.Open("config.json")
		defer file.Close()
		decoder := json.NewDecoder(file)
		err := decoder.Decode(&configuration)
		if err != nil {
			fmt.Println("error:", err)
		}
	}

	return configuration
}

func createOutputFile() *os.File {
	configuration := loadConfiguration()

	fName := configuration.Output.Filename
	file, err := os.Create(fName)
	if err != nil {
		log.Fatalf("Cannot create file %q: %s\n", fName, err)
	}

	return file
}

func regexpFromConfig(input []string) []*regexp.Regexp {
	var filters = make([]*regexp.Regexp, len(input)-1)
	for _, f := range input {
		re := regexp.MustCompile(f)

		filters = append(filters, re)
	}
	return filters
}

func main() {

	testUrlPtr := flag.String("testUrl", "", "A single URL. When provided, will show the output from that URL only.")

	flag.Parse()

	configuration := loadConfiguration()

	file := createOutputFile()
	defer file.Close()

	enc := json.NewEncoder(file)

	var options []colly.CollectorOption

	if configuration.Debug {
		options = append(options, colly.Debugger(&debug.LogDebugger{}))
	}

	if configuration.UserAgent != "" {
		log.Println("Adding user agent", configuration.UserAgent)
		options = append(options, colly.UserAgent(configuration.UserAgent))
	}

	options = append(options, colly.URLFilters(
		regexpFromConfig(configuration.Input.UrlFilters)...,
	))

	options = append(options, colly.DisallowedURLFilters(regexpFromConfig(configuration.Input.DisallowedUrlFilters)...))

	// Don't use the cache when testing
	if *testUrlPtr == "" {
		options = append(options, colly.CacheDir("./cache"))
	} else {
		log.Println("Running with test URL", *testUrlPtr)
	}
	options = append(options, colly.Async(true))

	c := colly.NewCollector(options...)

	c.SetRequestTimeout(time.Duration(configuration.Request.TimeoutInMs) * time.Millisecond)

	c.Limit(&colly.LimitRule{
		DomainGlob:  configuration.Request.DomainGlob,
		Parallelism: configuration.Request.Parallelism,
		Delay:       time.Duration(configuration.Request.DelayInMs) * time.Millisecond,
		RandomDelay: time.Duration(configuration.Request.RandomDelayInMs) * time.Millisecond,
	})

	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "*/*")
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Println("Something went wrong:", err, string(r.Body), r.Request.Headers)
	})

	c.OnHTML("html", func(e *colly.HTMLElement) {
		document := make(map[string]string)

		for key, selector := range configuration.Html.Selectors {
			var val string
			if strings.Contains(selector, "{{") {
				t := template.Must(template.New("selectorTpl").Funcs(sprig.TxtFuncMap()).Parse(selector))
				var tpl bytes.Buffer
				data := HtmlSelectorTemplateVars{Request: *e.Request, Response: *e.Response}
				err := t.Execute(&tpl, data)

				if err != nil {
					panic(err)
				}
				val = tpl.String()
			} else {
				val = strings.Join(ChildTexts(e, selector), " ")
			}
			document[key] = val
		}

		if *testUrlPtr == "" {
			// Write to JSONL as we gather the data, don't build it up in memory
			err := enc.Encode(document)

			if err != nil {
				log.Fatal(err)
			}

			e.ForEach("a[href]", func(_ int, el *colly.HTMLElement) {
				e.Request.Visit(el.Attr("href"))

			})
		} else {
			fmt.Println(document)
		}
	})

	if configuration.Pdf.Enabled {
		if _, err := os.Stat("pdf-cache/"); os.IsNotExist(err) {
			err := os.Mkdir("pdf-cache/", 0755)

			if err != nil {
				log.Fatal(err)
			}
		}

		c.OnResponse(func(resp *colly.Response) {
			ext := filepath.Ext(resp.Request.URL.Path)

			if ext == ".pdf" {
				pdfFile := "pdf-cache/" + filepath.Base(resp.Request.URL.Path)
				err := resp.Save(pdfFile)
				if err != nil {
					log.Fatal(err)
				}

				bodyResult, metaResult, err := ConvertPDFText(pdfFile)

				content := "PDF could not be parsed"
				var meta map[string]string

				if err != nil {
					log.Print(resp.Request.URL, err)
				} else {
					content = strings.Replace(bodyResult.body, "\n", " ", -1)
					doubleWhiteSpaceRegex := regexp.MustCompile(`[\s\p{Zs}]{2,}`)
					content = doubleWhiteSpaceRegex.ReplaceAllString(content, " ")
					meta = metaResult.meta
				}
				log.Print("selecting content")

				document := make(map[string]string)

				for key, selector := range configuration.Pdf.Selectors {
					var val string
					if strings.Contains(selector, "{{") {
						t := template.Must(template.New("selectorTpl").Funcs(sprig.TxtFuncMap()).Parse(selector))
						var tpl bytes.Buffer
						data := PdfSelectorTemplateVars{Request: *resp.Request, TextContent: content, Meta: meta}
						err := t.Execute(&tpl, data)

						if err != nil {
							panic(err)
						}
						val = tpl.String()
					}
					document[key] = val
				}

				if *testUrlPtr == "" {
					enc.Encode(document)
				} else {
					fmt.Println(document)
				}
			}
		})
	}

	if *testUrlPtr != "" {
		c.Visit(*testUrlPtr)
	} else {
		c.Visit(configuration.Input.StartUrl)
	}
	c.Wait()
}
