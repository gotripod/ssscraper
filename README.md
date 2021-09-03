Super-Simple Scraper
=

This a very thin layer on top of [Colly](http://go-colly.org/) which allows configuration from a JSON file. The output is JSONL which is ready to be imported into [Typesense](https://typesense.org).

Features
==

- Scrape HTML & PDF documents based on the configured selectors
- Selectors can use CSS selectors or template-based ones which have [sprig](http://masterminds.github.io/sprig/) functions available.

Configuration
==

See the [example configuration](https://github.com/gotripod/ssscraper/blob/master/config.example.json). Many of these options are directly copied to the Colly equivalents:

- http://go-colly.org/docs/introduction/configuration/
- https://pkg.go.dev/github.com/gocolly/colly?utm_source=godoc#Collector
- https://pkg.go.dev/github.com/gocolly/colly?utm_source=godoc#LimitRule

Running
==

We have an image on DockerHub, so after installing `Docker` and `jq`, something like this will work:

```
docker run -it -e "CONFIG=$(cat ./path/to/your/config.json | jq -r tostring)" gotripod/ssscraper
```

The manual method is:

```
docker build -t ssscraper .
docker run -v `pwd`:/go/src/app -it --rm --name ssscraper-ahoy ssscraper

# you're now in the docker container

cd src/app
go build
./ssscraper
```

Developing
==

Using VSCode, clone and open the repo directory with the Containers extension installed. 

Future ideas
==

- Webhook support - POST the output to a URL on completion
- Different output formats
- Custom weighting for selectors
- Extract the selector/template logic to a common function
- Add Word doc support

Sponsors
==

Built by [Go Tripod](https://gotripod.com), making the web as easy as one, two, three. Go Tripod build bespoke software solutions, and if you need a custom version of SS Scraper [please get in touch](https://gotripod.com/contact/).