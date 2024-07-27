package protocol

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

const (
	ipV6URINotationPrefix = "["
	ipV6URINotationSuffix = "]"
)

var ErrEmptyURL = errors.New("url to be parsed is empty")

// URL represents a URL with additional fields and methods.
type URL struct {
	SubName, Name, TLD, Port string
	IsDomain                 bool
	*url.URL
}

// String returns the string representation of the URL.
// It includes the scheme if `includeScheme` is true.
func (url URL) String(includeScheme bool) string {
	s := url.URL.String()
	if !includeScheme {
		s = RemoveScheme(s)
	}
	return s
}

// Domain returns the domain name of the URL. If includeSub is true and there is a subdomain, it includes the subdomain
// in the returned string. Otherwise, it only includes the domain.
func (url URL) Domain(includeSub bool) string {
	if includeSub && url.SubName != "" {
		return fmt.Sprintf("%s.%s.%s", url.SubName, url.Name, url.TLD)
	}
	return fmt.Sprintf("%s.%s", url.Name, url.TLD)
}

// NoWWW returns the domain name without the "www" subdomain.
// If the subdomain is not "www" or is empty, it returns the domain name as is.
// The returned domain name is a string in the format "subname.name.tld".
func (url URL) NoWWW() string {
	if url.SubName != "www" && url.SubName != "" {
		return fmt.Sprintf("%s.%s.%s", url.SubName, url.Name, url.TLD)
	}
	return fmt.Sprintf("%s.%s", url.Name, url.TLD)
}

// WWW returns the domain name with the "www" subdomain.
// If the subdomain is not "www", it returns the domain name as is.
// The returned domain name is a string in the format "subname.name.tld".
func (url URL) WWW() string {
	if url.SubName != "" {
		return fmt.Sprintf("%s.%s.%s", url.SubName, url.Name, url.TLD)
	}
	return fmt.Sprintf("%s.%s.%s", "www", url.Name, url.TLD)
}

// HTTPS returns the URL with HTTPS Scheme but leaves the URL itself untouched.
func (url URL) HTTPS() string {
	rememberScheme := url.Scheme
	url.Scheme = "https"
	httpsURL := url.String(true)
	url.Scheme = rememberScheme
	return httpsURL
}

// StripWWW returns the URL without "www" subdomain, but leaves the URL itself untouched.
// This function returns the whole URL with its path, in contrast to NoWWW().
func (url URL) StripWWW(includeScheme bool) string {
	if url.SubName == "www" {
		return strings.Replace(url.String(includeScheme), "www.", "", 1)
	}
	return url.String(includeScheme)
}

// StripQueryParams removes query parameters and fragments from the URL and returns
// the URL as a string. If includeScheme is true, it includes the scheme in the returned URL.
func (url URL) StripQueryParams(includeScheme bool) string {
	// Remember the original values of query parameters and fragments
	rememberRawQuery := url.RawQuery
	rememberFragment := url.Fragment
	rememberRawFragment := url.RawFragment

	// Clear the query parameters and fragments
	url.RawQuery = ""
	url.RawFragment = ""
	url.Fragment = ""

	// Get the URL without query parameters
	urlWithoutQuery := url.String(includeScheme)

	// Restore the original values of query parameters and fragments
	url.RawQuery = rememberRawQuery
	url.RawFragment = rememberRawFragment
	url.Fragment = rememberFragment

	return urlWithoutQuery
}

// IsLocal checks if the URL is a local address.
// It returns true if the URL's top-level domain (TLD) is "localhost" or if the URL's
// hostname resolves to a loopback IP address.
func (url URL) IsLocal() bool {
	ip := net.ParseIP(strings.TrimPrefix(strings.TrimSuffix(url.Name, ipV6URINotationSuffix), ipV6URINotationPrefix))
	return url.TLD == "localhost" || (ip != nil && ip.IsLoopback())
}

// Parse parses a string representation of a URL and returns a *URL and error.
// It mirrors the net/url.Parse function but returns a tld.URL, which contains extra fields.
func Parse(urlString string) (*URL, error) {
	urlString = strings.TrimSpace(urlString)

	// if the url to be parsed is empty after trimming, we return an error
	if len(urlString) == 0 {
		return nil, ErrEmptyURL
	}

	urlString = AddDefaultScheme(urlString)
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return nil, fmt.Errorf("could not parse url: %w", err)
	}
	// always lowercase subdomain.domain.tld (host property)
	parsedURL.Host = strings.ToLower(parsedURL.Host)
	if parsedURL.Host == "" {
		return &URL{URL: parsedURL}, nil
	}
	dom, port := domainPort(parsedURL.Host)
	var domName, tld, sub string
	ip := net.ParseIP(strings.TrimPrefix(strings.TrimSuffix(dom, ipV6URINotationSuffix), ipV6URINotationPrefix))
	switch {
	case ip != nil:
		domName = dom
	case dom == "localhost":
		tld = dom
	default:
		etld1, err := publicsuffix.EffectiveTLDPlusOne(dom)
		if err != nil {
			return nil, fmt.Errorf("failed to extract eTLD+1: %w", err)
		}
		i := strings.Index(etld1, ".")
		domName = etld1[0:i]
		tld = etld1[i+1:]
		sub = ""
		if rest := strings.TrimSuffix(dom, "."+etld1); rest != dom {
			sub = rest
		}
	}
	urlString, err = idna.ToASCII(dom)
	if err != nil {
		return nil, fmt.Errorf("failed to convert domain to ASCII: %w", err)
	}
	return &URL{
		SubName:  sub,
		Name:     domName,
		TLD:      tld,
		Port:     port,
		URL:      parsedURL,
		IsDomain: IsDomainName(urlString),
	}, nil
}

// FromParsed mirrors the net/url.Parse function,
// but instead of returning a *url.URL, it returns a *URL,
// which is a struct that contains additional fields.
//
// The function first checks if the parsedUrl.Host field is empty.
// If it is empty, it returns a *URL with the URL field set to parsedUrl
// and all other fields set to their zero values.
//
// If the parsedUrl.Host field is not empty, it extracts the domain and port
// using the domainPort function.
//
// It then calculates the effective top-level domain plus one (etld+1)
// using the publicsuffix.EffectiveTLDPlusOne function.
//
// The etld+1 is then split into the domain name (domName) and the top-level domain (tld).
//
// It further determines the subdomain (sub) by checking if the domain is a subdomain of the etld+1.
//
// The domain name (domName) is then converted to ASCII using the idna.ToASCII function.
//
// Finally, it returns a *URL with the extracted values and the URL field set to parsedUrl.
// The IsDomain field is set to the result of the IsDomainName function called with the ASCII domain name.
// The SubName field is set to sub, the Name field is set to domName, and the T.
func FromParsed(parsedURL *url.URL) (*URL, error) {
	if parsedURL.Host == "" {
		return &URL{URL: parsedURL}, nil
	}
	dom, port := domainPort(parsedURL.Host)
	// etld+1
	etld1, err := publicsuffix.EffectiveTLDPlusOne(dom)
	if err != nil {
		return nil, fmt.Errorf("failed to extract eTLD+1: %w", err)
	}
	// convert to domain name, and tld
	i := strings.Index(etld1, ".")
	domName := etld1[0:i]
	tld := etld1[i+1:]
	// and subdomain
	sub := ""
	if rest := strings.TrimSuffix(dom, "."+etld1); rest != dom {
		sub = rest
	}
	asciiDom, err := idna.ToASCII(dom)
	if err != nil {
		return nil, fmt.Errorf("failed to convert domain to ASCII: %w", err)
	}
	return &URL{
		SubName:  sub,
		Name:     domName,
		TLD:      tld,
		Port:     port,
		URL:      parsedURL,
		IsDomain: IsDomainName(asciiDom),
	}, nil
}

// domainPort extracts the domain and port from the host part of a URL.
// If the host contains a port, it returns the domain without the port and the port as strings.
// If the host does not contain a port, it returns the domain and an empty string for the port.
// If the host is all numeric characters, it returns the host itself and an empty string for the port.
// Note that the net/url package should prevent the string from being all numeric characters.
func domainPort(host string) (string, string) {
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			return host[:i], host[i+1:]
		} else if host[i] < '0' || host[i] > '9' {
			return host, ""
		}
	}
	// will only land here if the string is all digits,
	// net/url should prevent that from happening
	return host, ""
}

// IsDomainName checks if a string represents a valid domain name.
//
// It follows the rules specified in RFC 1035 and RFC 3696 for domain name validation.
//
// The input string is first processed with the RemoveScheme function to remove any scheme prefix.
// The domain name is then split into labels using the dot separator.
// The function checks that the number of labels is at least 2 and that the total length of the string is between 1 and
// 254 characters.
//
// The function iterates over the characters of the string and performs checks based on the character type.
// Valid characters include letters (a-zA-Z), digits (0-9), underscore (_), and hyphen (-).
// Each label can contain up to 63 characters and the last label cannot end with a hyphen.
// The function also checks that the byte before a dot or a hyphen is not a dot or a hyphen, respectively.
// Non-numeric characters are tracked to ensure the presence of at least one non-numeric character in the domain name.
//
// If any of the checks fail, the function returns false. Otherwise, it returns true.
//
// Example usage:
// s := "mail.google.com"
// isValid := IsDomainName(s).
func IsDomainName(name string) bool { //nolint:cyclop
	name = RemoveScheme(name)
	// See RFC 1035, RFC 3696.
	// Presentation format has dots before every label except the first, and the
	// terminal empty label is optional here because we assume fully-qualified
	// (absolute) input. We must therefore reserve space for the first and last
	// labels' length octets in wire format, where they are necessary and the
	// maximum total length is 255.
	// So our _effective_ maximum is 253, but 254 is not rejected if the last
	// character is a dot.
	split := strings.Split(name, ".")

	// Need a TLD and a domain.
	if len(split) < 2 { //nolint:gomnd
		return false
	}
	l := len(name)
	if l == 0 || l > 254 || l == 254 && name[l-1] != '.' {
		return false
	}

	last := byte('.')
	nonNumeric := false // true once we've seen a letter or hyphen
	partlen := 0
	for i := 0; i < len(name); i++ {
		char := name[i]
		switch {
		default:
			return false
		case 'a' <= char && char <= 'z' || 'A' <= char && char <= 'Z' || char == '_':
			nonNumeric = true
			partlen++
		case '0' <= char && char <= '9':
			// fine
			partlen++
		case char == '-':
			// Byte before dash cannot be dot.
			if last == '.' {
				return false
			}
			partlen++
			nonNumeric = true
		case char == '.':
			// Byte before dot cannot be dot, dash.
			if last == '.' || last == '-' {
				return false
			}
			if partlen > 63 || partlen == 0 {
				return false
			}
			partlen = 0
		}
		last = char
	}
	if last == '-' || partlen > 63 {
		return false
	}

	return nonNumeric
}

// RemoveScheme removes the scheme from a URL string.
// If the URL string includes a scheme (e.g., "http://"), the scheme will be removed and the remaining string will be returned.
// If the URL string includes a default scheme (e.g., "//"), the default scheme will be removed and the remaining string will be returned.
// If the URL string does not include a scheme, the original string will be returned unchanged.
func RemoveScheme(s string) string {
	if strings.Contains(s, "://") {
		return removeScheme(s)
	}
	if strings.Contains(s, "//") {
		return removeDefaultScheme(s)
	}
	return s
}

// add default scheme if string does not include a scheme.
func AddDefaultScheme(s string) string {
	if !strings.Contains(s, "//") ||
		(!strings.Contains(s, "//") && !strings.Contains(s, ":") && !strings.Contains(s, "@")) {
		return addDefaultScheme(s)
	}
	return s
}

func AddScheme(s, scheme string) string {
	if scheme == "" {
		return AddDefaultScheme(s)
	}
	if strings.Index(s, "//") == -1 {
		return fmt.Sprintf("%s://%s", scheme, s)
	}
	return s
}

// addDefaultScheme returns a new string with a default scheme added.
// The default scheme format is "//<original_string>".
func addDefaultScheme(s string) string {
	return fmt.Sprintf("//%s", s)
}

// removeDefaultScheme removes the default scheme from a string.
func removeDefaultScheme(s string) string {
	return s[index(s, "//"):]
}

func removeScheme(s string) string {
	return s[index(s, "://"):]
}

// index returns the starting index of the first occurrence of the specified scheme in the given string.
// If the scheme is not found, it returns -1.
// The returned index is incremented by the length of the scheme to obtain the starting position of the remaining string.
func index(s, scheme string) int {
	return strings.Index(s, scheme) + len(scheme)
}
