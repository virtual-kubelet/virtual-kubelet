package auth

// HWS API Gateway Signature
// Analog to AWS Signature Version 4, with some HWS specific parameters
// Please refer to: http://docs.aws.amazon.com/general/latest/gr/signature-version-4.html

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"
)

// BasicDateFormat and BasicDateFormatShort define aws-date format
const (
	BasicDateFormat      = "20060102T150405Z"
	BasicDateFormatShort = "20060102"
	TerminationString    = "sdk_request"
	Algorithm            = "SDK-HMAC-SHA256"
	PreSKString          = "SDK"
	HeaderXDate          = "x-sdk-date"
	HeaderDate           = "date"
	HeaderHost           = "host"
	HeaderAuthorization  = "Authorization"
	HeaderContentSha256  = "x-sdk-content-sha256"
	// todo: use the region and service.
	DefaultRegion  = "default"
	DefaultService = "apigateway"
)

func shouldEscape(c byte) bool {
	if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '_' || c == '-' || c == '~' || c == '.' {
		return false
	}
	return true
}
func escape(s string) string {
	hexCount := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if shouldEscape(c) {
			hexCount++
		}
	}

	if hexCount == 0 {
		return s
	}

	t := make([]byte, len(s)+2*hexCount)
	j := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case shouldEscape(c):
			t[j] = '%'
			t[j+1] = "0123456789ABCDEF"[c>>4]
			t[j+2] = "0123456789ABCDEF"[c&15]
			j += 3
		default:
			t[j] = s[i]
			j++
		}
	}
	return string(t)
}

func hmacsha256(key []byte, data string) ([]byte, error) {
	h := hmac.New(sha256.New, []byte(key))
	if _, err := h.Write([]byte(data)); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

// Build a CanonicalRequest from a regular request string
//
// See http://docs.aws.amazon.com/general/latest/gr/sigv4-create-canonical-request.html
// CanonicalRequest =
//  HTTPRequestMethod + '\n' +
//  CanonicalURI + '\n' +
//  CanonicalQueryString + '\n' +
//  CanonicalHeaders + '\n' +
//  SignedHeaders + '\n' +
//  HexEncode(Hash(RequestPayload))
func CanonicalRequest(r *http.Request) (string, error) {
	var hexencode string
	var err error
	if hex := r.Header.Get(HeaderContentSha256); hex != "" {
		hexencode = hex
	} else {
		data, err := RequestPayload(r)
		if err != nil {
			return "", err
		}
		hexencode, err = HexEncodeSHA256Hash(data)
	}
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s", r.Method, CanonicalURI(r), CanonicalQueryString(r), CanonicalHeaders(r), SignedHeaders(r), hexencode), err
}

// CanonicalURI returns request uri
func CanonicalURI(r *http.Request) string {
	pattens := strings.Split(r.URL.Path, "/")
	var uri []string
	for _, v := range pattens {
		switch v {
		case "":
			continue
		case ".":
			continue
		case "..":
			if len(uri) > 0 {
				uri = uri[:len(uri)-1]
			}
		default:
			uri = append(uri, escape(v))
		}
	}
	urlpath := "/"
	if len(uri) > 0 {
		urlpath = urlpath + strings.Join(uri, "/") + "/"
	}
	return urlpath
}

// CanonicalQueryString
func CanonicalQueryString(r *http.Request) string {
	var a []string
	for key, value := range r.URL.Query() {
		k := escape(key)
		for _, v := range value {
			var kv string
			if v == "" {
				kv = k
			} else {
				kv = fmt.Sprintf("%s=%s", k, escape(v))
			}
			a = append(a, kv)
		}
	}
	sort.Strings(a)
	query := strings.Join(a, "&")
	r.URL.RawQuery = query
	return query
}

// CanonicalHeaders
func CanonicalHeaders(r *http.Request) string {
	var a []string
	for key, value := range r.Header {
		sort.Strings(value)
		var q []string
		for _, v := range value {
			q = append(q, trimString(v))
		}
		a = append(a, strings.ToLower(key)+":"+strings.Join(q, ","))
	}
	a = append(a, HeaderHost+":"+r.Host)
	sort.Strings(a)
	return fmt.Sprintf("%s\n", strings.Join(a, "\n"))
}

// SignedHeaders
func SignedHeaders(r *http.Request) string {
	var a []string
	for key := range r.Header {
		a = append(a, strings.ToLower(key))
	}
	a = append(a, HeaderHost)
	sort.Strings(a)
	return fmt.Sprintf("%s", strings.Join(a, ";"))
}

// RequestPayload
func RequestPayload(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return []byte(""), nil
	}
	b, err := ioutil.ReadAll(r.Body)
	r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
	return b, err
}

// Return the Credential Scope. See http://docs.aws.amazon.com/general/latest/gr/sigv4-create-string-to-sign.html
func CredentialScope(t time.Time, regionName, serviceName string) string {
	return fmt.Sprintf("%s/%s/%s/%s", t.UTC().Format(BasicDateFormatShort), regionName, serviceName, TerminationString)
}

// Create a "String to Sign". See http://docs.aws.amazon.com/general/latest/gr/sigv4-create-string-to-sign.html
func StringToSign(canonicalRequest, credentialScope string, t time.Time) string {
	hash := sha256.New()
	hash.Write([]byte(canonicalRequest))
	return fmt.Sprintf("%s\n%s\n%s\n%x",
		Algorithm, t.UTC().Format(BasicDateFormat), credentialScope, hash.Sum(nil))
}

// Generate a "signing key" to sign the "String To Sign". See http://docs.aws.amazon.com/general/latest/gr/sigv4-calculate-signature.html
func GenerateSigningKey(secretKey, regionName, serviceName string, t time.Time) ([]byte, error) {

	key := []byte(PreSKString + secretKey)
	var err error
	dateStamp := t.UTC().Format(BasicDateFormatShort)
	data := []string{dateStamp, regionName, serviceName, TerminationString}
	for _, d := range data {
		key, err = hmacsha256(key, d)
		if err != nil {
			return nil, err
		}
	}
	return key, nil
}

// Create the HWS Signature. See http://docs.aws.amazon.com/general/latest/gr/sigv4-calculate-signature.html
func SignStringToSign(stringToSign string, signingKey []byte) (string, error) {
	hm, err := hmacsha256(signingKey, stringToSign)
	return fmt.Sprintf("%x", hm), err
}

// HexEncodeSHA256Hash returns hexcode of sha256
func HexEncodeSHA256Hash(body []byte) (string, error) {
	hash := sha256.New()
	if body == nil {
		body = []byte("")
	}
	_, err := hash.Write(body)
	return fmt.Sprintf("%x", hash.Sum(nil)), err
}

// Get the finalized value for the "Authorization" header. The signature parameter is the output from SignStringToSign
func AuthHeaderValue(signature, accessKey, credentialScope, signedHeaders string) string {
	return fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s", Algorithm, accessKey, credentialScope, signedHeaders, signature)
}

func trimString(s string) string {
	var trimedString []byte
	inQuote := false
	var lastChar byte
	s = strings.TrimSpace(s)
	for _, v := range []byte(s) {
		if byte(v) == byte('"') {
			inQuote = !inQuote
		}
		if lastChar == byte(' ') && byte(v) == byte(' ') && !inQuote {
			continue
		}
		trimedString = append(trimedString, v)
		lastChar = v
	}
	return string(trimedString)
}

type Signer interface {
	Sign(*http.Request) error
}

// Signature HWS meta
type SignerHws struct {
	AppKey    string
	AppSecret string
	Region    string
	Service   string
}

// SignRequest set Authorization header
func (s *SignerHws) Sign(r *http.Request) error {
	var t time.Time
	var err error
	var dt string
	if dt = r.Header.Get(HeaderXDate); dt != "" {
		t, err = time.Parse(BasicDateFormat, dt)
	} else if dt = r.Header.Get(HeaderDate); dt != "" {
		t, err = time.Parse(time.RFC1123, dt)
	}
	if err != nil || dt == "" {
		r.Header.Del(HeaderDate)
		t = time.Now()
		r.Header.Set(HeaderXDate, t.UTC().Format(BasicDateFormat))
	}
	canonicalRequest, err := CanonicalRequest(r)
	if err != nil {
		return err
	}
	Region := DefaultRegion
	Service := DefaultService
	if s.Region != "" {
		Region = s.Region
	}
	if s.Service != "" {
		Service = s.Service
	}
	credentialScope := CredentialScope(t, Region, Service)
	stringToSign := StringToSign(canonicalRequest, credentialScope, t)
	key, err := GenerateSigningKey(s.AppSecret, Region, Service, t)
	if err != nil {
		return err
	}
	signature, err := SignStringToSign(stringToSign, key)
	if err != nil {
		return err
	}
	signedHeaders := SignedHeaders(r)
	authValue := AuthHeaderValue(signature, s.AppKey, credentialScope, signedHeaders)
	r.Header.Set(HeaderAuthorization, authValue)
	return nil
}
