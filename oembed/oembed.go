package oembed

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// Oembed contains list of available oembed items (official endpoints)
type Oembed struct {
	items []*Item
}

// Endpoint contains single endpoint to check against
type Endpoint struct {
	URL       string   `json:"url"`
	Discovery bool     `json:"discovery,omitempty"`
	Schemes   []string `json:"schemes,omitempty"`
}

// Provider contains a single provider which can have multiple endpoints
type Provider struct {
	Name      string     `json:"provider_name"`
	URL       string     `json:"provider_url"`
	Endpoints []Endpoint `json:"endpoints"`
}

// Item contains data for a schema
type Item struct {
	EndpointURL  string
	ProviderName string
	ProviderURL  string
	regex        *regexp.Regexp
}

// ComposeURL returns url of oembed resource ready to be queried
func (item *Item) ComposeURL(u string) string {
	return item.EndpointURL + url.QueryEscape(u)
}

// FetchOembed return oembed info from an url containing it
func (item *Item) FetchOembed(u string, client *http.Client) (*Info, error) {
	resURL := item.ComposeURL(u)

	var resp *http.Response
	var err error

	if client != nil {
		resp, err = client.Get(resURL)
	} else {
		resp, err = http.Get(resURL)
	}

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode > 200 {
		return &Info{Status: resp.StatusCode}, nil
	}

	reader := io.LimitReader(resp.Body, 40000) // 40 KB max

	info := NewInfo()
	err = info.FillFromJSON(reader)

	if err != nil {
		return nil, err
	}

	if len(info.URL) == 0 {
		info.URL = u
	}

	if len(info.ProviderURL) == 0 {
		info.ProviderURL = item.ProviderURL
	}

	if len(info.ProviderName) == 0 {
		info.ProviderName = item.ProviderName
	}

	return info, nil
}

// MatchURL tests if given url applies to the endpoint
func (item *Item) MatchURL(url string) bool {
	return item.regex.MatchString(strings.Trim(url, "\r\n"))
}

// NewOembed creates Oembed instance
func NewOembed() *Oembed {
	return &Oembed{}
}

// ParseProviders build oembed endpoint list based on provided json stream
func (o *Oembed) ParseProviders(buf io.Reader) error {
	var providers []Provider

	data, err := ioutil.ReadAll(buf)

	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &providers)

	if err != nil {
		return err
	}

	var items []*Item

	for _, provider := range providers {
		for _, endpoint := range provider.Endpoints {
			if len(endpoint.Schemes) == 0 {
				endpoint.Schemes = append(endpoint.Schemes, strings.TrimRight(provider.URL, "/")+"/*")
			}
			for _, schema := range endpoint.Schemes {
				or := &Item{ProviderName: provider.Name, ProviderURL: provider.URL}
				or.EndpointURL = o.prepareEndpointURL(endpoint.URL)
				or.regex = o.convertSchemaURL2Regexp(schema)
				items = append(items, or)
			}
		}
	}

	o.items = items

	return nil
}

// FindItem returns Oembed item based on provided url
func (o *Oembed) FindItem(url string) *Item {
	for _, or := range o.items {
		if or.MatchURL(url) {
			return or
		}
	}

	return nil
}

// TODO: add more intelligent parameters parsing
func (o *Oembed) prepareEndpointURL(url string) string {
	url = strings.Replace(url, "{format}", "json", -1)
	url = strings.Replace(url, "/*", "", -1) // hack for Ora TV.. wtf they put in?
	if strings.IndexRune(url, '?') == -1 {
		url += "?format=json&url="
	} else {
		url += "&format=json&url="
	}

	return url
}

// TODO: precompile and move out regexes from the function
func (o *Oembed) convertSchemaURL2Regexp(url string) *regexp.Regexp {
	// domain replacements
	url = strings.Replace(url, "?", "\\?", -1)
	re1 := regexp.MustCompile("^(https?://[^/]*?)\\*(.+)$")
	url = re1.ReplaceAllString(url, "${1}[^/]%?${2}")
	re2 := regexp.MustCompile("^(https?://[^/]*?/.*?)\\*(.+)$")
	url = re2.ReplaceAllString(url, "${1}.%?${2}")
	re3 := regexp.MustCompile("^(https?://.*?)\\*$")
	url = re3.ReplaceAllString(url, "${1}.%")
	re4 := regexp.MustCompile("^http://")
	url = re4.ReplaceAllString(url, "https?://")
	url = strings.Replace(url, "%", "*", -1)
	////
	res, err := regexp.Compile("^" + url + "$")

	if err != nil {
		panic(err)
	}

	return res
}
