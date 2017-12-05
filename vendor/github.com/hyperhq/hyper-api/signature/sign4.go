/*
   Based on the AWS Signature Algorithm Sign4 http://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html
   Based on the Implementation of https://github.com/smartystreets/go-aws-auth
     - Both Sign and Check
     - hostname of Hyper
     - change header X-AMZ- to X-Hyper-
     - changed normuri, treat // as /
*/
package signature

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	headerPrefix      = "X-Hyper-"
	headerDate        = "X-Hyper-Date"
	headerContentHash = "X-Hyper-Content-Sha256"
	headerAuthz       = "Authorization"

	metaAlgorithm = "HYPER-HMAC-SHA256"

	keyPartsPrefix  = "HYPER"
	keyPartsRequest = "hyper_request"

	timeFormatV4 = "20060102T150405Z"

	reqExpiration = 5 * time.Minute
)

type AuthnHeader struct {
	Algorithm    string
	AccessKey    string
	Scope        string
	SignedHeader string
	Signature    string
	Date         string
}

func Signiture4(secretKey string, req *http.Request, header *AuthnHeader, region string) (bool, error) {
	meta := &metadata{
		algorithm:       header.Algorithm,
		credentialScope: header.Scope,
		signedHeaders:   header.SignedHeader,
		date:            header.Date,
		region:          region,
		service:         "hyper",
	}

	hashedCanonReq, ok := canonicalRequestV4FromMeta(req, meta)
	if !ok {
		return false, errors.New("payload check error")
	}

	stringToSign := metaToSignV4(req, hashedCanonReq, meta)

	signingKey := signingKeyV4(secretKey, meta.date, meta.region, meta.service)
	signature := signatureV4(signingKey, stringToSign)
	return signature == header.Signature, nil
}

func Sign4(accessKey, secretKey string, req *http.Request, region string) *http.Request {

	prepareRequestV4(req)
	meta := &metadata{}

	// Task 1
	hashedCanonReq := hashedCanonicalRequestV4(req, meta)

	// Task 2
	stringToSign := stringToSignV4(req, hashedCanonReq, meta, region)

	// Task 3
	signingKey := signingKeyV4(secretKey, meta.date, meta.region, meta.service)
	signature := signatureV4(signingKey, stringToSign)

	req.Header.Set(headerAuthz, buildAuthHeaderV4(accessKey, signature, meta))

	return req
}

// Build Request Steps
func prepareRequestV4(request *http.Request) *http.Request {
	necessaryDefaults := map[string]string{
		"Content-Type": "application/json",
		headerDate:     timestampV4(),
	}

	for header, value := range necessaryDefaults {
		if request.Header.Get(header) == "" {
			request.Header.Set(header, value)
		}
	}

	if request.URL.Path == "" {
		request.URL.Path += "/"
	}

	return request
}

func hashedCanonicalRequestV4(request *http.Request, meta *metadata) string {
	// TASK 1. http://docs.aws.amazon.com/general/latest/gr/sigv4-create-canonical-request.html

	payload := readAndReplaceBody(request)
	payloadHash := hashSHA256(payload)
	request.Header.Set(headerContentHash, payloadHash)

	// Set this in header values to make it appear in the range of headers to sign
	request.Header.Set("Host", request.URL.Host)

	var sortedHeaderKeys []string
	for key := range request.Header {
		switch key {
		case "Content-Type", "Content-Md5", "Host":
		default:
			if !strings.HasPrefix(key, headerPrefix) {
				continue
			}
		}
		sortedHeaderKeys = append(sortedHeaderKeys, strings.ToLower(key))
	}
	sort.Strings(sortedHeaderKeys)

	var headersToSign string
	for _, key := range sortedHeaderKeys {
		value := strings.TrimSpace(request.Header.Get(key))
		if key == "host" {
			//Hyper(AWS) does not include port in signing request.
			if strings.Contains(value, ":") {
				split := strings.Split(value, ":")
				port := split[1]
				if port == "80" || port == "443" {
					value = split[0]
				}
			}
		}
		headersToSign += key + ":" + value + "\n"
	}
	meta.signedHeaders = concat(";", sortedHeaderKeys...)
	canonicalRequest := concat("\n", request.Method, normuri(request.URL.Path), normquery(request.URL.Query()), headersToSign, meta.signedHeaders, payloadHash)

	return hashSHA256([]byte(canonicalRequest))
}

func canonicalRequestV4FromMeta(request *http.Request, meta *metadata) (string, bool) {
	payload := readPayload(request)
	payloadHash := hashSHA256(payload)
	if request.Header.Get(headerContentHash) != payloadHash {
		return "", false
	}
	var headersToSign string
	for _, hdr := range strings.Split(meta.signedHeaders, ";") {
		value := strings.TrimSpace(request.Header.Get(hdr))
		if hdr == "host" {
			//Hyper(AWS) does not include port in signing request.
			if strings.Contains(value, ":") {
				split := strings.Split(value, ":")
				port := split[1]
				if port == "80" || port == "443" {
					value = split[0]
				}
			}
		}
		headersToSign += hdr + ":" + value + "\n"
	}
	canonicalRequest := concat("\n", request.Method, normuri(request.URL.Path), normquery(request.URL.Query()), headersToSign, meta.signedHeaders, payloadHash)
	return canonicalRequest, true
}

func stringToSignV4(request *http.Request, hashedCanonReq string, meta *metadata, region string) string {
	// TASK 2. http://docs.aws.amazon.com/general/latest/gr/sigv4-create-string-to-sign.html

	requestTs := request.Header.Get(headerDate)

	meta.algorithm = metaAlgorithm
	meta.service, meta.region = serviceAndRegion(request.Host, region)
	meta.date = tsDateV4(requestTs)
	meta.credentialScope = concat("/", meta.date, meta.region, meta.service, keyPartsRequest)

	return concat("\n", meta.algorithm, requestTs, meta.credentialScope, hashedCanonReq)
}

func metaToSignV4(request *http.Request, hashedCanonReq string, meta *metadata) string {
	return concat("\n", meta.algorithm, request.Header.Get(headerDate), meta.credentialScope, hashedCanonReq)
}

func signingKeyV4(secretKey, date, region, service string) []byte {
	kDate := hmacSHA256([]byte(keyPartsPrefix+secretKey), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, keyPartsRequest)
	return kSigning
}

func signatureV4(signingKey []byte, stringToSign string) string {
	// TASK 3. http://docs.aws.amazon.com/general/latest/gr/sigv4-calculate-signature.html

	return hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
}

func buildAuthHeaderV4(accessKey, signature string, meta *metadata) string {
	credential := accessKey + "/" + meta.credentialScope

	return meta.algorithm +
		" Credential=" + credential +
		", SignedHeaders=" + meta.signedHeaders +
		", Signature=" + signature
}

// Check Request Steps
func validateExpire(req *http.Request) bool {
	dh := req.Header.Get(headerDate)
	if dh == "" {
		return false
	}
	date, err := time.ParseInLocation(timeFormatV4, dh, time.UTC)
	if err != nil {
		return false
	}
	if date.Add(reqExpiration).Before(time.Now().UTC()) {
		return false
	}
	return true
}

// Details
type metadata struct {
	algorithm       string
	credentialScope string
	signedHeaders   string
	date            string
	region          string
	service         string
}

func timestampV4() string {
	return time.Now().UTC().Format(timeFormatV4)
}

func readAndReplaceBody(request *http.Request) []byte {
	if request.Body == nil {
		return []byte{}
	}
	payload, _ := ioutil.ReadAll(request.Body)
	request.Body = ioutil.NopCloser(bytes.NewReader(payload))
	return payload
}

func readPayload(req *http.Request) []byte {
	if req.Body == nil {
		return []byte{}
	}
	payload, _ := ioutil.ReadAll(req.Body)
	return payload
}

func hmacSHA256(key []byte, content string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(content))
	return mac.Sum(nil)
}

func hashSHA256(content []byte) string {
	h := sha256.New()
	h.Write(content)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func concat(delim string, str ...string) string {
	return strings.Join(str, delim)
}

func normuri(uri string) string {
	parts := []string{}
	for _, s := range strings.Split(uri, "/") {
		if s == "" {
			//bypass empty path segments
			continue
		}
		parts = append(parts, encodePathFrag(s))
	}
	return strings.Join(parts, "/")
}

func encodePathFrag(s string) string {
	hexCount := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if shouldEscape(c) {
			hexCount++
		}
	}
	t := make([]byte, len(s)+2*hexCount)
	j := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if shouldEscape(c) {
			t[j] = '%'
			t[j+1] = "0123456789ABCDEF"[c>>4]
			t[j+2] = "0123456789ABCDEF"[c&15]
			j += 3
		} else {
			t[j] = c
			j++
		}
	}
	return string(t)
}

func shouldEscape(c byte) bool {
	if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' {
		return false
	}
	if '0' <= c && c <= '9' {
		return false
	}
	if c == '-' || c == '_' || c == '.' || c == '~' {
		return false
	}
	return true
}

func normquery(v url.Values) string {
	queryString := v.Encode()

	// Go encodes a space as '+' but Amazon requires '%20'. Luckily any '+' in the
	// original query string has been percent escaped so all '+' chars that are left
	// were originally spaces.

	return strings.Replace(queryString, "+", "%20", -1)
}

// serviceAndRegion parsers a hostname to find out which ones it is.
func serviceAndRegion(host, r string) (service string, region string) {
	// These are the defaults if the hostname doesn't suggest something else
	region = r
	service = "hyper"

	// region.hyper.sh
	if strings.HasSuffix(host, ".hyper.sh") {
		parts := strings.SplitN(host, ".", 2)
		if parts[1] == "hyper.sh" {
			region = parts[0]
		}
	}
	// no more service yet

	return
}

func tsDateV4(timestamp string) string {
	return timestamp[:8]
}
