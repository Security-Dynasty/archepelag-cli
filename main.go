package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"
)

const usage = `Archepelag CLI

Usage:
  archepelag products list [--search TEXT] [--json]
  archepelag products get ID_OR_SLUG [--json]

Set ARCHEPELAG_SERVER_URL to your Archepelag instance URL and
ARCHEPELAG_TOKEN to an API token from Settings > Tokens.
`

type product struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 || isHelp(args) || (args[0] == "products" && (len(args) == 1 || isHelp(args[1:]))) {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if len(args) < 2 || args[0] != "products" || (args[1] != "list" && args[1] != "get") {
		fmt.Fprintln(stderr, "Unknown command. Run archepelag --help.")
		return 2
	}

	command, options := args[1], args[2:]
	if isHelp(options) {
		fmt.Fprint(stdout, usage)
		return 0
	}
	flags := flag.NewFlagSet("products "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprint(stderr, usage) }
	jsonOutput := flags.Bool("json", false, "Print product data as JSON")
	var search *string
	if command == "list" {
		search = flags.String("search", "", "Search products")
	} else if len(options) > 0 && !strings.HasPrefix(options[0], "-") {
		// Accept the JSON flag after the product reference as well.
		options = append(append([]string{}, options[1:]...), options[0])
	}
	if err := flags.Parse(options); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	ref := flags.Arg(0)
	if command == "get" && (flags.NArg() != 1 || !validProductRef(ref)) {
		fmt.Fprintln(stderr, "Provide one product UUID or slug. Run archepelag --help.")
		return 2
	}
	if command == "list" && flags.NArg() != 0 {
		fmt.Fprintln(stderr, "Use --search TEXT to filter the product list.")
		return 2
	}

	api, err := newAPI(getenv)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if command == "list" {
		var products []json.RawMessage
		products, err = api.listProducts(*search)
		if err == nil {
			err = printProducts(stdout, products, *jsonOutput)
		}
	} else {
		var response struct {
			Product json.RawMessage `json:"product"`
		}
		err = api.get("/products/"+url.PathEscape(ref), nil, &response)
		if err == nil {
			err = printProduct(stdout, response.Product, *jsonOutput)
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func isHelp(args []string) bool {
	return len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help")
}

func validProductRef(ref string) bool {
	return ref != "" && ref != "." && ref != ".." && !strings.ContainsAny(ref, "/\\") &&
		strings.IndexFunc(ref, unicode.IsControl) < 0
}

func printProducts(out io.Writer, raw []json.RawMessage, asJSON bool) error {
	products := make([]product, 0, len(raw))
	for _, element := range raw {
		item, err := decodeProduct(element)
		if err != nil {
			return err
		}
		products = append(products, item)
	}
	if asJSON {
		return printJSON(out, raw)
	}
	if len(products) == 0 {
		_, err := fmt.Fprintln(out, "No products found.")
		return err
	}
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tKEY\tNAME")
	for _, item := range products {
		fmt.Fprintf(table, "%s\t%s\t%s\n", textField(item.ID), textField(item.Key), textField(item.Name))
	}
	return table.Flush()
}

func printProduct(out io.Writer, raw json.RawMessage, asJSON bool) error {
	item, err := decodeProduct(raw)
	if err != nil {
		return err
	}
	if asJSON {
		return printJSON(out, raw)
	}
	summary := fmt.Sprintf("Name: %s\nID:   %s\nKey:  %s\n", textField(item.Name), textField(item.ID), textField(item.Key))
	if item.Description != "" {
		summary += "\n" + textField(item.Description) + "\n"
	}
	_, err = io.WriteString(out, summary)
	return err
}

func decodeProduct(raw json.RawMessage) (product, error) {
	var item product
	if err := json.Unmarshal(raw, &item); err != nil || item.ID == "" {
		return product{}, errors.New("API returned invalid product data")
	}
	return item, nil
}

func printJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func textField(value string) string {
	return strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return ' '
		}
		return character
	}, value)
}

type apiClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newAPI(getenv func(string) string) (*apiClient, error) {
	baseURL, err := url.Parse(strings.TrimSpace(getenv("ARCHEPELAG_SERVER_URL")))
	if err != nil || baseURL.Hostname() == "" || (baseURL.Scheme != "https" && baseURL.Scheme != "http") ||
		baseURL.User != nil || baseURL.RawQuery != "" || baseURL.ForceQuery || baseURL.Fragment != "" {
		return nil, errors.New("Set ARCHEPELAG_SERVER_URL to the instance URL, for example https://archepelag.example")
	}
	token := strings.TrimSpace(getenv("ARCHEPELAG_TOKEN"))
	if token == "" || strings.IndexFunc(token, unicode.IsSpace) >= 0 || strings.IndexFunc(token, unicode.IsControl) >= 0 {
		return nil, errors.New("Set ARCHEPELAG_TOKEN to a valid API token")
	}
	return &apiClient{
		baseURL: strings.TrimRight(baseURL.String(), "/") + "/api/v1",
		token:   token,
		http: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (api *apiClient) listProducts(search string) ([]json.RawMessage, error) {
	products := make([]json.RawMessage, 0)
	query := url.Values{"pageSize": {"100"}}
	if search != "" {
		query.Set("search", search)
	}
	for page := 1; ; page++ {
		query.Set("page", strconv.Itoa(page))
		var result struct {
			Products   []json.RawMessage `json:"products"`
			Pagination struct {
				Page       int `json:"page"`
				TotalPages int `json:"totalPages"`
			} `json:"pagination"`
		}
		if err := api.get("/products", query, &result); err != nil {
			return nil, err
		}
		if result.Products == nil || result.Pagination.Page < 1 ||
			result.Pagination.Page > page || result.Pagination.TotalPages < result.Pagination.Page {
			return nil, errors.New("API returned invalid product pagination")
		}
		// The API clamps the requested page when the catalog shrinks during paging.
		if result.Pagination.Page < page {
			return products, nil
		}
		products = append(products, result.Products...)
		if page == result.Pagination.TotalPages {
			return products, nil
		}
	}
}

func (api *apiClient) get(path string, query url.Values, result any) error {
	endpoint := api.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return errors.New("Could not build the API request. Check ARCHEPELAG_SERVER_URL")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+api.token)
	response, err := api.http.Do(request)
	if err != nil {
		return fmt.Errorf("Could not reach the Archepelag API: %w", err)
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(response.Body).Decode(result); err != nil {
			return errors.New("API returned invalid JSON")
		}
		return nil
	case http.StatusUnauthorized:
		return errors.New("HTTP 401: check that ARCHEPELAG_TOKEN is valid and has not expired")
	case http.StatusForbidden:
		return errors.New("HTTP 403: the token does not have access")
	case http.StatusNotFound:
		return errors.New("HTTP 404: check the product UUID or slug and the instance URL")
	default:
		return fmt.Errorf("Archepelag API returned HTTP %d", response.StatusCode)
	}
}
