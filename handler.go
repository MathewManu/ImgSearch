package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"
)

/*
 * Following struct can be used to represent the whole
 * response from the model.
 * This struct can be used for unmarshalling the response.
 *
 * Mainly concerned about
 *    1. Concepts & probability
 *    2. Status.Code
 */
type JsonResp struct {
	Outputs []struct {
		ID        string `json:"id"`
		CreatedAt string `json:"created_at"`
		Data      struct {
			Concepts []struct {
				ID    string  `json:"id"`
				AppID string  `json:"app_id"`
				Name  string  `json:"name"`
				Value float64 `json:"value"`
			} `json:"concepts"`
		} `json:"data"`
		Input struct {
			ID   string `json:"id"`
			Data struct {
				Image struct {
					URL string `json:"url"` // https://samples.clarifai.com/metro-north.jpg
				} `json:"image"`
			} `json:"data"`
		} `json:"input"`
		Model struct {
			ID           string `json:"id"`
			AppID        string `json:"app_id"`
			CreatedAt    string `json:"created_at"`
			DisplayName  string `json:"display_name"`
			ModelVersion struct {
				ID        string `json:"id"`
				CreatedAt string `json:"created_at"`
				Status    struct {
					Code        int64  `json:"code"`
					Description string `json:"description"`
				} `json:"status"`
			} `json:"model_version"`
			Name       string `json:"name"`
			OutputInfo struct {
				Type    string `json:"type"`
				TypeExt string `json:"type_ext"`
			} `json:"output_info"`
		} `json:"model"`
		Status struct {
			Code        int64  `json:"code"`
			Description string `json:"description"`
		} `json:"status"`
	} `json:"outputs"`
	Status struct {
		Code        int64  `json:"code"`
		Description string `json:"description"`
	} `json:"status"`
}

type HttpResponse struct {
	url      string
	response *JsonResp
	err      error
}

/*
 * This method takes an HTTP response.
 * Unmarshal that to a JsonResp struct and returns.
 */
func getResponse(httpRespBody []byte) (*JsonResp, error) {
	var s = new(JsonResp)
	err := json.Unmarshal(httpRespBody, &s)
	if err != nil {
		fmt.Println("Error !!! :", err)
	}
	return s, err
}

/*
 * This method can be used to add new urls to add more images
 * to the system.
 */
func fetch_image_urls(externalImageUrl string) []string {

	resp, err := http.Get(externalImageUrl)
	if err != nil {
		fmt.Println("Couln't fetch image urls !!! ")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	urls := strings.Split(string(body), "\n")
	return urls
}

/*
 * make async calls to clarifai server for each images.
 * --> Requests are batched. (20) // can go upto 30?
 * --> Also, added 1 sec delay to avoid any 11005
 *
 */
func asyncFetchImageData(urls []string) []*HttpResponse {

	ch := make(chan *HttpResponse)

	responses := []*HttpResponse{}
	var iterations, batch_count int
	var buf bytes.Buffer

	for indx, url := range urls {

		buf.WriteString(getImageUrl_body(url))
		/* BATCH 20 requests */
		if (indx > 0 && indx%20 == 0) || (indx == len(urls)-1) {
			batch_count++

			result := get_http_header(buf)
			buf.Reset()

			/* sleep */
			iterations++
			if iterations == 4 {
				iterations = 0
				time.Sleep(1000 * time.Millisecond)
			}

			go func(url string, result string) {

				modelUrl := "https://api.clarifai.com/v2/models/aaa03c23b3724a16a56b629203edc62c/outputs"

				var jsonStr = []byte(result)
				req, err := http.NewRequest("POST", modelUrl, bytes.NewBuffer(jsonStr))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Key b97503375c7b45dc99ed9552001fc989")
				client := &http.Client{}
				resp, err := client.Do(req)

				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					panic(err.Error())
				}
				s, err := getResponse([]byte(body))

				ch <- &HttpResponse{url, s, err}
				if err != nil && resp != nil && resp.StatusCode == http.StatusOK {
					resp.Body.Close()
				}
			}(url, result)
		}
	}
	for {
		select {
		case r := <-ch:
			if r.err != nil {
				fmt.Println("with an error", r.err)
			}
			responses = append(responses, r)

			if len(responses) == batch_count {
				return responses
			}
		case <-time.After(200 * time.Millisecond):
			fmt.Printf(".")
		}
	}
	return responses
}

/* data structure for image tags */
type tagsUrl struct {
	prob float64
	uri  string
}
type tagsUrls []tagsUrl

func (d tagsUrls) Len() int           { return len(d) }
func (d tagsUrls) Less(i, j int) bool { return d[i].prob > d[j].prob }
func (d tagsUrls) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }

var tagsUrlMap = map[string]tagsUrls{}
var metaData = map[string]tagsUrls{}

func Process(imageRepoUrl string) {

	img_urls := fetch_image_urls(imageRepoUrl)
	img_urls_quoted := []string{}
	for _, url := range img_urls {
		img_urls_quoted = append(img_urls_quoted, `"`+url+`"`)
	}

	allresp := asyncFetchImageData(img_urls_quoted)
	for _, resp := range allresp {
		obj := resp.response

		if obj.Status.Code == 10000 {

			/* for each req in a batched msg */
			for _, req := range obj.Outputs {
				//fmt.Println("url : ", req.Input.Data.Image.URL)
				for _, concept := range req.Data.Concepts {
					tagsUrlMap[concept.Name] = append(tagsUrlMap[concept.Name], tagsUrl{concept.Value, req.Input.Data.Image.URL})
					metaData[req.Input.Data.Image.URL] = append(metaData[req.Input.Data.Image.URL], tagsUrl{concept.Value, concept.Name})

				}
			}

		}

	}
	/* As of now all the urls are kept in memory
	 * can be reduced to a fixed value say 10 or 20 */
	for _, v := range tagsUrlMap {
		sort.Sort(v)
	}

	for _, v := range metaData {
		sort.Sort(v)
	}

}

type QueryResponse struct {
	url  string
	tags []string
}

/*
 * This gets called from rest.go
 * Input: search query
 * Response: QueryResponse struct
 *			 --> url
 *			 --> 4 tags similar to search query
 */
func Get_n_image_urls(keyword string, n int) []QueryResponse {

	var resp = []QueryResponse{}
	imgList, ok := tagsUrlMap[keyword]

	if ok {
		if len(imgList) < n {
			n = len(imgList)
		}
		nItems := imgList[:n]
		/* for each image, get suggestions */
		for _, k := range nItems {
			tagSuggestions := get_tag_suggestions(k, keyword)
			resp = append(resp, QueryResponse{k.uri, tagSuggestions})
		}
	}
	return resp
}

/*
 * get 3 suggestions related to the queried string
 */
func get_tag_suggestions(k tagsUrl, keyword string) []string {
	var tagSuggestions = []string{}
	tagList, ok := metaData[k.uri]
	if ok {
		count := 0
		for _, t := range tagList {
			if t.uri != keyword {
				tagSuggestions = append(tagSuggestions, t.uri)
				count++
			}
			if count == 3 {
				break
			}
		}
	}
	return tagSuggestions
}

/*
 * Get HTTP request header generated for a given URL
 * This should be changd to a struct probably, so that
 * we can add more parameters easily
 */
func getImageUrl_body(imageUrl string) string {
	query := `
	      {
		"data": {
		  "image": {
		    "url": ` + imageUrl +
		`}
		}
	      },`
	return query
}

func get_http_header(buff bytes.Buffer) string {
	result := buff.String()
	result = strings.TrimSuffix(result, ",")
	result = `{ "inputs": 
			[` + result +
		`]
	    		}`
	return result
}
