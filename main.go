package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

//fetchResult is
type fetchResult struct {
	version  string
	title    string
	headings map[string]int
	urls     []string
}

//parse returns *goquery documents
func parse(url string) (*goquery.Document, error) {
	res, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer res.Body.Close()

	//check status code
	if res.StatusCode != http.StatusOK {
		log.Fatalf("Error response status code was %d", res.StatusCode)
	}

	// Create a goquery document from the HTTP response
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal("Error loading HTTP response body ", err)
	}
	return doc, nil
}

func main() {
	// baseURL := "http://symbolic.com/"
	baseURL := os.Args[1]
	if baseURL == "" {
		log.Fatalln("missing url")
	}

	doc, err := parse(baseURL)
	if err != nil || doc == nil {
		return
	}

	//collect fetchResult from site
	fresult := fetch(doc)
	fmt.Println("FetchResult", fresult)

	//analyse urls found
	//findinternals finds internal links
	findinternals := func(s string) bool {
		return strings.HasPrefix(s, baseURL) || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "#")
	}
	internals := Filter(fresult.urls, findinternals)
	fmt.Printf("found %d internal links and %d external \n", len(internals), len(fresult.urls)-len(internals))

	//containsLoginByURL checks if internal links contain login (could be done with regex as well)
	containsLoginByURL := func(il string) bool {
		s := strings.ToUpper(il)
		return strings.Contains(s, "LOGIN") || strings.Contains(s, "SIGNIN")
	}
	login := Filter(internals, containsLoginByURL)
	if len(login) == 0 {
		fmt.Println("no login found")
	} else {
		fmt.Printf("found %d login links\n", len(login))
	}

	// make channel
	c := make(chan string)

	//pingLinks concurrently
	for _, u := range fresult.urls {
		go pingLink(u, c)
	}
	// receive inaccessible links from channel
	ia := []string{}
	for l := range c {
		ia = append(ia, l)
	}
	fmt.Printf("found %d inaccessible links", len(ia))

}

//Filter finds internal links
func Filter(ss []string, f func(string) bool) (filtered []string) {
	for _, s := range ss {
		if f(s) {
			filtered = append(filtered, s)
		}
	}
	return
}

//pingLink checks if link is accessible and sends inaccessible links to channel
func pingLink(link string, c chan string) {
	_, err := http.Get(link)
	if err != nil {
		// fmt.Println(link, "down")
		c <- link //send to channel
		return
	}
	time.Sleep(5 * time.Second)
	close(c)
}

//fetch finds elements on website and returns a fetchresult
func fetch(doc *goquery.Document) *fetchResult {
	fr := fetchResult{}

	v, err := versionReader(doc)
	if err != nil {
		fmt.Println("Error loading version", err)
	}
	fr.version = v
	fr.title = doc.Find("title").Contents().Text()
	fr.headings = getHeadings(doc)
	fr.urls = getURLs(doc)

	return &fr
}

// getHeadings finds all headings H1-H6 and returns map of headings count by level
func getHeadings(doc *goquery.Document) map[string]int {
	hs := map[string]int{
		"h1": 0,
		"h2": 0,
		"h3": 0,
		"h4": 0,
		"h5": 0,
		"h6": 0,
	}
	for i := 1; i <= 6; i++ {
		str := strconv.Itoa(i)
		doc.Find("h" + str).Each(func(i int, s *goquery.Selection) {
			hs["h"+str] = +1
		})
	}
	return hs
}

//getURLs finds all urls and returns slice of unique urls
//the contains check could be removed if urls do not need to be unique
func getURLs(doc *goquery.Document) []string {
	foundUrls := []string{}
	doc.Find("a").Each(func(i int, s *goquery.Selection) {
		u, _ := s.Attr("href")
		if !Contains(foundUrls, u) {
			foundUrls = append(foundUrls, u)
		}
	})
	return foundUrls
}

//Contains returns true if slice already contains url
func Contains(urls []string, url string) bool {
	for _, v := range urls {
		if v == url {
			return true
		}
	}
	return false
}

//checks HTML version and returns first match
func versionReader(doc *goquery.Document) (string, error) {
	doctypes := map[string]string{
		"HTML 5":                 `<!DOCTYPE html>`,
		"HTML 4.01 Strict":       `"-//W3C//DTD HTML 4.01//EN"`,
		"HTML 4.01 Transitional": `"-//W3C//DTD HTML 4.01 Transitional//EN"`,
		"HTML 4.01 Frameset":     `"-//W3C//DTD HTML 4.01 Frameset//EN"`,
		"XHTML 1.0 Strict":       `"-//W3C//DTD XHTML 1.0 Strict//EN"`,
		"XHTML 1.0 Transitional": `"-//W3C//DTD XHTML 1.0 Transitional//EN"`,
		"XHTML 1.0 Frameset":     `"-//W3C//DTD XHTML 1.0 Frameset//EN"`,
		"XHTML 1.1":              `"-//W3C//DTD XHTML 1.1//EN"`,
	}
	//e.g. http://symbolic.com/  =>  XHTML 1.0 Transitional
	html, err := doc.Html()
	if err != nil {
		return "", err
	}
	version := ""
	for d, m := range doctypes {
		if strings.Contains(html, m) {
			version = d
		}
	}
	return version, nil
}
