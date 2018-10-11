package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type PageData struct {
	PageTitle string
	DivData   interface{}
}

func main() {

	defaultUrl := "https://s3.amazonaws.com/clarifai-data/backend/api-take-home/images.txt"

	mux := http.NewServeMux()
	mux.HandleFunc("/", GetHandler)
	mux.HandleFunc("/search", PostHandler)

	 /*
	  *	Following method can exposed for adding new image repository for tagging
	  */

	/* mux.HandleFunc("/addNewRepo", PostHandlerAddNewRepo) */
	

	Process(defaultUrl)

	fmt.Println("Up and running !!")
	http.ListenAndServe(GetPort(), mux)
}

func GetPort() string {
	var port = os.Getenv("PORT")
	if port == "" {
		port = "3030"
		fmt.Println("INFO: No PORT env variable detected, Starting at: " + port)
	}
	return ":" + port
}

func GetHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("home.html"))
	data := PageData{
		PageTitle: "",
	}
	tmpl.Execute(w, data)
}

func PostHandler(w http.ResponseWriter, r *http.Request) {
	
	tmpl := template.Must(template.ParseFiles("results.html"))
	if r.Method == "POST" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body",
				http.StatusInternalServerError)
		}

		formQuery := string(body)
		tags := strings.Split(formQuery, "=")

		topImageUrls := Get_n_image_urls(tags[1], 10)

		/* create div section 3 column format w3-third style 
		 * This should be changed to JSON format so that more info can be easily added!!!
		*/
		var div1, div2, div3 string
		for indx, k := range topImageUrls {

			if indx%3 == 0 {
				div1 += getImgDiv(k)
			}
			if indx%3 == 1 {
				div2 += getImgDiv(k)
			}
			if indx%3 == 2 {
				div3 += getImgDiv(k)
			}
		}

		divContent := getDivContent(div1, div2, div3)
		data := PageData{
			PageTitle: tags[1],
			DivData:   template.HTML(divContent),
		}

		tmpl.Execute(w, data)
	} else {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
	}
}

/*
 * html content to push 
 */

func getSuggestionLink(s []string) string {
	str := ""
	for _, k := range s {
		str += `<a  class="button4">` + k + `</a>`
	}
	return str
}

func getImgDiv(queryResp QueryResponse) string {
	str := `<img src="` + queryResp.url + `" style="width:100%">`
	str += `<div class="container"> <p>` + getSuggestionLink(queryResp.tags) + `</p></div>`
	return str
}
func getDivContent(d1 string, d2 string, d3 string) string {
	str := `<div class="w3-third container">` + d1 + `</div>
	<div class="w3-third container">` + d2 + `</div>
	<div class="w3-third container">` + d3 + `</div>`
	return str
}
