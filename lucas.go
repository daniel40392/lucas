package main

import (
	"encoding/json"
	"os"
	"fmt"
	"strings"
	"github.com/gocolly/colly"
	"github.com/fatih/color"
	"database/sql"
	_ "github.com/lib/pq"
	"strconv"
	"github.com/joho/godotenv"
)

type Clothing struct {
	Name					string
	Code					string
	Description		string
	Price					float64
}

func dbWrite(product Clothing) {

	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=disable",
    os.Getenv("HOST"),
		os.Getenv("PORT"),
		os.Getenv("USER"),
		os.Getenv("DBNAME"))

	db, err := sql.Open("postgres", psqlInfo)
  if err != nil {
    panic(err)
  }
  defer db.Close()

  err = db.Ping()
  if err != nil {
    panic(err)
  }

	sqlStatement := `
	INSERT INTO floryday (product, code, description, price)
	VALUES ($1, $2, $3, $4)`
	_, err = db.Exec(sqlStatement, product.Name, product.Code, product.Description, product.Price)

	if err != nil {
		color.Red("[DB] Failed Write: %s", product.Name)
	  panic(err)
	}
	color.Green("[DB] Successful Write: %s", product.Name)
}

func main() {

	// loading config
	err := godotenv.Load()
  if err != nil {
    color.Red("Error loading .env file")
  }

	// setting up colly collector
	c := colly.NewCollector(
		// colly.AllowedDomains("https://www.floryday.com/"),
		colly.CacheDir(".floryday_cache"),
  	// colly.MaxDepth(5), // keeping crawling limited for our initial experiments,
		// colly.UserAgent("Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2228.0 Safari/537.36")
  )

	// clothing detail scraping collector
	detailCollector := c.Clone()

	// defaulting to array of 200 results
	clothes := make([]Clothing, 0, 200)

	// Find and visit all links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		// hardcoded urls to skip -> to be optimized -> perhaps map links from external file...
		if !strings.HasPrefix(link, "/?country_code") || strings.Index(link, "/cart.php") > -1 ||
		strings.Index(link, "/login.php") > -1 || strings.Index(link, "/cart.php") > -1 ||
		strings.Index(link, "/account") > -1 || strings.Index(link, "/privacy-policy.html") > -1 {
			return
		}
		// scrape the page
		e.Request.Visit(link)
	})

	// printing visiting message for debug purposes
	c.OnRequest(func(r *colly.Request) {
		color.Blue("Visiting", r.URL.String(), "\n")
	})

	// TODO filter this better a[href] is way too broad -> may need regex
	c.OnHTML(`a[href]`, func(e *colly.HTMLElement) {
		clothingURL := e.Request.AbsoluteURL(e.Attr("href"))
		// links provided need to be better filtered
		// hardcoding one value only to work here for now...
		if strings.Contains(clothingURL, "-Dress-"){
			// Activate detailCollector
			// Setting default country_code for currency purposes
			color.Magenta("Commencing Crawl for %s", clothingURL + "?country_code=IE")
			detailCollector.Visit(clothingURL + "?country_code=IE")
		} else {
			color.Red("Validation Failed -> Cancelling Crawl for %s", clothingURL + "?country_code=IE")
			return
		}
	})

	// Extract details of the clothing
	detailCollector.OnHTML(`html`, func(e *colly.HTMLElement) {

		// TODO secure variables with default error strings in event values are missing
		title := e.ChildText(".prod-name")
		code := strings.Split(e.ChildText(".prod-item-code"), "#")[1]

		// price parsing & reformatting
		initialprice := e.ChildText(".currency-prices")
		pricenosymbol := strings.TrimSuffix(initialprice," €")
		stringPrice := strings.Replace(pricenosymbol, ",", ".", 1)
		price, err := strconv.ParseFloat(stringPrice, 64) // conversion to float64
		if err != nil {
	    color.Red("err in parsing price -> %s", err)
	  }


		// desecription requires more refined parsing into subsections
		description := strings.TrimSpace(e.ChildText(".grid-uniform"))

		clothing := Clothing{
			Name: 					title,
			Code: 					code,
			Description: 		description,
			Price:					price,
		}

		// writing as we go to DB
		// TODO optiize to handle bulk array uplaods instead
		dbWrite(clothing)

		// appending to our output array...
		clothes = append(clothes, clothing)
	})

	// start scraping at our seed address
	c.Visit(os.Getenv("SEED_ADDRESS"))

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	// Dump json to the standard output
	enc.Encode(clothes)

}
