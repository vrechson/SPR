package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/schollz/progressbar/v3"
)

type Parameter struct {
	Name     string                 `json:"name"`
	In       string                 `json:"in"`
	Required bool                   `json:"required"`
	Schema   map[string]interface{} `json:"schema"`
	Example  interface{}            `json:"example"`
}

type Operation struct {
	Parameters  []Parameter            `json:"parameters"`
	RequestBody map[string]interface{} `json:"requestBody"`
}

type PathItem struct {
	Get        *Operation  `json:"get"`
	Post       *Operation  `json:"post"`
	Put        *Operation  `json:"put"`
	Delete     *Operation  `json:"delete"`
	Patch      *Operation  `json:"patch"`
	Parameters []Parameter `json:"parameters"`
}

type OpenAPI struct {
	Paths map[string]PathItem `json:"paths"`
}

// Static UUID for all requests
const staticUUID = "11111111-1111-1111-1111-111111111111"

// Integer fuzzing values
var intFuzzValues = []int{1, 10, 100, 1000, 10000, 100000, 1000000, 10000000,
	2, 20, 200, 2000, 20000, 200000, 2000000, 20000000,
	5, 50, 500, 5000, 50000, 500000, 5000000, 50000000,
	9, 90, 900, 9000, 90000, 900000, 9000000, 90000000}

func getDummyValue(schema map[string]interface{}, example interface{}, intFuzz bool, fuzzValue int, paramName string, paramOverrides map[string]string) interface{} {
	// Check if there's an override for this parameter
	if override, exists := paramOverrides[paramName]; exists && !intFuzz {
		return override
	}

	if example != nil {
		return example
	}

	schemaType, _ := schema["type"].(string)
	schemaFormat, _ := schema["format"].(string)

	// Check for UUID format first
	if schemaFormat == "uuid" {
		return staticUUID
	}

	// If type is missing but properties exist, treat as object
	if schemaType == "" {
		if properties, ok := schema["properties"].(map[string]interface{}); ok {
			result := make(map[string]interface{})
			for key, prop := range properties {
				propMap := prop.(map[string]interface{})
				result[key] = getDummyValue(propMap, nil, intFuzz, fuzzValue, key, paramOverrides)
			}
			return result
		}
	}

	switch schemaType {
	case "string":
		return "test_string"
	case "integer":
		if intFuzz {
			if override, exists := paramOverrides[paramName]; exists {
				return override
			}
			return fuzzValue
		}
		return 1000
	case "number":
		return 1.0
	case "boolean":
		return true
	case "array":
		items := schema["items"].(map[string]interface{})
		return []interface{}{getDummyValue(items, nil, intFuzz, fuzzValue, paramName, paramOverrides)}
	case "object":
		result := make(map[string]interface{})
		if properties, ok := schema["properties"].(map[string]interface{}); ok {
			for key, prop := range properties {
				propMap := prop.(map[string]interface{})
				result[key] = getDummyValue(propMap, nil, intFuzz, fuzzValue, key, paramOverrides)
			}
		}
		return result
	default:
		// Return empty object instead of string if properties exist
		if properties, ok := schema["properties"].(map[string]interface{}); ok {
			result := make(map[string]interface{})
			for key, prop := range properties {
				propMap := prop.(map[string]interface{})
				result[key] = getDummyValue(propMap, nil, intFuzz, fuzzValue, key, paramOverrides)
			}
			return result
		}
		return "test_value"
	}
}

func normalizeHost(host string) string {
	return strings.TrimRight(host, "/")
}

func hasIntegerParams(operation *Operation, pathParams []Parameter) bool {
	allParams := append(pathParams, operation.Parameters...)

	// Check path and query parameters
	for _, param := range allParams {
		if schemaType, ok := param.Schema["type"].(string); ok && schemaType == "integer" {
			return true
		}
	}

	// Check request body
	if operation.RequestBody != nil {
		if content, ok := operation.RequestBody["content"].(map[string]interface{}); ok {
			if jsonContent, ok := content["application/json"]; ok {
				if schema, ok := jsonContent.(map[string]interface{})["schema"].(map[string]interface{}); ok {
					if schemaType, ok := schema["type"].(string); ok && schemaType == "integer" {
						return true
					}
				}
			}
		}
	}

	return false
}

func main() {
	swaggerFile := flag.String("swagger", "", "Path to OpenAPI/Swagger file")
	host := flag.String("host", "", "Target host")
	proxy := flag.String("proxy", "http://127.0.0.1:8080", "Proxy URL")
	methods := flag.String("methods", "GET", "Comma-separated list of HTTP methods to test (GET,POST,PUT,DELETE,PATCH)")
	intFuzzing := flag.Bool("int-fuzzing", false, "Enable integer parameter fuzzing")
	headers := flag.String("H", "", "Headers to add to requests (can be specified multiple times)")
	threads := flag.Int("threads", 10, "Number of concurrent threads")
	verbose := flag.Bool("v", false, "Verbose output")
	var paramOverrides arrayFlags
	flag.Var(&paramOverrides, "param-override", "Override parameter values in format param=value (can be specified multiple times)")
	flag.Parse()

	if *swaggerFile == "" || *host == "" {
		fmt.Println("Please provide swagger file path and host")
		os.Exit(1)
	}

	// Parse allowed methods
	allowedMethods := strings.Split(strings.ToUpper(*methods), ",")
	methodsMap := make(map[string]bool)
	for _, m := range allowedMethods {
		methodsMap[strings.TrimSpace(m)] = true
	}

	// Parse headers
	headerMap := make(map[string]string)
	if *headers != "" {
		for _, h := range strings.Split(*headers, ",") {
			parts := strings.SplitN(h, ":", 2)
			if len(parts) == 2 {
				headerMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	// Parse param overrides
	paramOverrideMap := make(map[string]string)
	for _, override := range paramOverrides {
		parts := strings.SplitN(override, "=", 2)
		if len(parts) == 2 {
			paramOverrideMap[parts[0]] = parts[1]
		}
	}

	// Read and parse swagger file
	data, err := ioutil.ReadFile(*swaggerFile)
	if err != nil {
		fmt.Printf("Error reading swagger file: %v\n", err)
		os.Exit(1)
	}

	var api OpenAPI
	if err := json.Unmarshal(data, &api); err != nil {
		fmt.Printf("Error parsing swagger file: %v\n", err)
		os.Exit(1)
	}

	// Configure proxy and disable TLS verification
	proxyURL, err := url.Parse(*proxy)
	if err != nil {
		fmt.Printf("Error parsing proxy URL: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	normalizedHost := normalizeHost(*host)

	// Count total requests to be made
	totalRequests := 0
	for _, pathItem := range api.Paths {
		operations := map[string]*Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"DELETE": pathItem.Delete,
			"PATCH":  pathItem.Patch,
		}

		for method, operation := range operations {
			if operation == nil || !methodsMap[method] {
				continue
			}

			hasIntegers := hasIntegerParams(operation, pathItem.Parameters)
			if *intFuzzing && hasIntegers {
				totalRequests += len(intFuzzValues)
			} else {
				totalRequests++
			}
		}
	}

	// Create progress bar if not in verbose mode
	var bar *progressbar.ProgressBar
	if !*verbose {
		bar = progressbar.Default(int64(totalRequests))
	}

	// Create request channel and wait group
	requestChan := make(chan struct {
		method    string
		url       string
		body      []byte
		headerMap map[string]string
	})
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < *threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for req := range requestChan {
				request, err := http.NewRequest(req.method, req.url, strings.NewReader(string(req.body)))
				if err != nil {
					if *verbose {
						fmt.Printf("Error creating request: %v\n", err)
					}
					if !*verbose {
						bar.Add(1)
					}
					continue
				}

				// Add default Content-Type header
				request.Header.Set("Content-Type", "application/json")

				// Add custom headers
				for key, value := range req.headerMap {
					request.Header.Set(key, value)
				}

				// Send request
				resp, err := client.Do(request)
				if err != nil {
					if *verbose {
						fmt.Printf("Error sending request: %v\n", err)
					}
					if !*verbose {
						bar.Add(1)
					}
					continue
				}
				resp.Body.Close()

				if *verbose {
					fmt.Printf("%s %s -> %d\n", req.method, req.url, resp.StatusCode)
				} else {
					bar.Add(1)
				}
			}
		}()
	}

	// Process each path
	for path, pathItem := range api.Paths {
		operations := map[string]*Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"DELETE": pathItem.Delete,
			"PATCH":  pathItem.Patch,
		}

		for method, operation := range operations {
			if operation == nil || !methodsMap[method] {
				continue
			}

			hasIntegers := hasIntegerParams(operation, pathItem.Parameters)
			fuzzValues := []int{1000}
			if *intFuzzing && hasIntegers {
				fuzzValues = intFuzzValues
			}

			for _, fuzzValue := range fuzzValues {
				// Combine path and operation parameters
				allParams := append(pathItem.Parameters, operation.Parameters...)

				// Build request URL with path and query parameters
				requestURL := normalizedHost + path
				queryParams := url.Values{}
				bodyParams := make(map[string]interface{})

				for _, param := range allParams {
					value := getDummyValue(param.Schema, param.Example, *intFuzzing, fuzzValue, param.Name, paramOverrideMap)

					// For methods that typically have a body, put parameters in body instead of query
					if (method == "POST" || method == "PUT" || method == "PATCH") && param.In == "query" {
						bodyParams[param.Name] = value
					} else {
						switch param.In {
						case "path":
							requestURL = strings.Replace(requestURL, "{"+param.Name+"}", fmt.Sprint(value), -1)
						case "query":
							queryParams.Add(param.Name, fmt.Sprint(value))
						}
					}
				}

				if len(queryParams) > 0 {
					requestURL += "?" + queryParams.Encode()
				}

				// Create request body
				var body []byte
				if operation.RequestBody != nil {
					content := operation.RequestBody["content"].(map[string]interface{})
					if jsonContent, ok := content["application/json"]; ok {
						schema := jsonContent.(map[string]interface{})["schema"].(map[string]interface{})
						bodyData := getDummyValue(schema, nil, *intFuzzing, fuzzValue, "", paramOverrideMap)
						body, _ = json.Marshal(bodyData)
					}
				} else if method == "POST" || method == "PUT" || method == "PATCH" {
					// If no request body defined but method typically needs one, use collected body params or empty object
					if len(bodyParams) > 0 {
						body, _ = json.Marshal(bodyParams)
					} else {
						body = []byte("{}")
					}
				}

				// Send request to worker pool
				requestChan <- struct {
					method    string
					url       string
					body      []byte
					headerMap map[string]string
				}{
					method:    method,
					url:       requestURL,
					body:      body,
					headerMap: headerMap,
				}
			}
		}
	}

	// Close channel and wait for workers to finish
	close(requestChan)
	wg.Wait()
}

// arrayFlags allows a flag to be specified multiple times
type arrayFlags []string

func (i *arrayFlags) String() string {
	return strings.Join(*i, ",")
}

func (i *arrayFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}
