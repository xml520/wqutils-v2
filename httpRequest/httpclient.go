package httpRequest

import (
	"bytes"
	"encoding/json"
	"github.com/tidwall/gjson"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var (
	defaultHttpClient = new(http.Client)
	jsonHeader        = Header{"content-type": "application/json; charset=utf-8", "Accept": "application/json"}
)

// NewClient 创建客户端
func NewClient(defaultParams *RequestParams) *Request {
	return &Request{defaultParams: defaultParams, params: NewRequestParams()}
}

// NewRequestParams 创建请求参数
func NewRequestParams() *RequestParams {
	return &RequestParams{header: make(Header)}
}

type Header map[string]string
type Body interface {
	string | []byte
}
type MyMap[KEY int | string, VALUE float32 | float64] map[KEY]VALUE

// RequestParams 请求参数配置
type RequestParams struct {
	header        Header
	cookie        string
	responseHooks []func(response *Response) error
	client        *http.Client
}

func (rp *RequestParams) SetHeader(k string, v string) *RequestParams {
	rp.header[k] = v
	return rp
}
func (rp *RequestParams) SetHeaders(h Header) *RequestParams {
	for k, v := range h {
		rp.SetHeader(k, v)
	}
	return rp
}
func (rp *RequestParams) SetCookie(v string) *RequestParams {
	rp.cookie = v
	return rp
}
func (rp *RequestParams) SetProxy(url *url.URL) *RequestParams {
	rp.client = &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(url),
		},
	}
	return rp
}
func (rp *RequestParams) setRequest(hr *http.Request) *http.Request {
	for k, v := range rp.header {
		hr.Header.Set(k, v)
	}
	if len(rp.cookie) != 0 {
		hr.Header.Set("cookie", rp.cookie)
	}
	return hr
}

// Request 请求
type Request struct {
	defaultParams *RequestParams
	params        *RequestParams
	rawRequest    *http.Request
}

func (r *Request) SetHeaders(h Header) *Request {
	r.params.SetHeaders(h)
	return r
}
func (r *Request) SetHeader(k string, v string) *Request {
	r.params.SetHeader(k, v)
	return r
}
func (r *Request) SetCookie(v string) *Request {
	r.params.SetCookie(v)
	return r
}
func (r *Request) SetProxy(url *url.URL) *Request {
	r.params.SetProxy(url)
	return r
}

func (r *Request) JsonRequest(method string, httpUrl string, body any) (res *Response) {
	return r.SetHeaders(jsonHeader).Request(method, httpUrl, body)
}

func (r *Request) Request(method string, httpUrl string, body any) (res *Response) {
	res = &Response{request: r}
	var (
		data          io.Reader
		requestClient *http.Client
	)
	{
		switch body.(type) {
		case string:
			data = strings.NewReader(body.(string)) // bytes.NewReader([]byte(body.(string)))
		case []byte:
			data = bytes.NewReader(body.([]byte))
		case io.Reader:
			data = body.(io.Reader)
		case io.ReadCloser:
			data = body.(io.ReadCloser)
		default:
			var _body []byte
			if _body, res.Error = json.Marshal(body); res.Error != nil {
				return
			} else {
				data = bytes.NewReader(_body)
			}
		}
	}
	if r.rawRequest, res.Error = http.NewRequest(strings.ToUpper(method), httpUrl, data); res.Error != nil {
		return
	}
	if r.defaultParams != nil {
		r.defaultParams.setRequest(r.rawRequest)
		if r.defaultParams.client != nil {
			requestClient = r.defaultParams.client
		}
	}
	r.defaultParams.setRequest(r.rawRequest)
	if r.defaultParams.client != nil {
		requestClient = r.defaultParams.client
	}
	if requestClient != nil {
		res.rawResponse, res.Error = requestClient.Do(r.rawRequest)
	} else {
		res.rawResponse, res.Error = defaultHttpClient.Do(r.rawRequest)
	}
	if res.Error != nil {
		return
	}
	if res.data, res.Error = io.ReadAll(res.rawResponse.Body); res.Error != nil {
		return
	}
	res.rawResponse.Body.Close()
	if r.params.responseHooks != nil {
		for i, _ := range r.params.responseHooks {
			if res.Error = r.params.responseHooks[i](res); res.Error != nil {
				return
			}
		}
	} else {
		for i, _ := range r.defaultParams.responseHooks {
			if res.Error = r.defaultParams.responseHooks[i](res); res.Error != nil {
				return
			}
		}
	}
	return
}

//Response 响应
type Response struct {
	Error       error
	request     *Request
	rawResponse *http.Response
	data        []byte
}

func (r *Response) Json(t any) *Response {
	if r.Error != nil {
		return r
	}
	r.Error = json.Unmarshal(r.data, t)
	return r
}
func (r *Response) GJson(path string) gjson.Result {
	return gjson.GetBytes(r.data, path)
}
func (r *Response) Bytes() []byte {
	return r.data
}
func (r *Response) String() string {
	return string(r.data)
}
func (r *Response) Response() *http.Response {
	return r.rawResponse
}
func (r *Response) Request() *http.Request {
	return r.request.rawRequest
}
func (r *Response) ExportHeader(field ...string) (h Header) {
	h = make(Header)
	var rh = r.Response().Header
	if len(field) == 0 {
		for k, v := range rh {
			h[k] = strings.Join(v, ";")
		}
	} else {
		for i, _ := range field {
			h[field[i]] = rh.Get(field[i])
		}
	}
	return
}
func (r *Response) ExportCookie(field ...string) (c string) {
	var (
		cookie    = r.Response().Cookies()
		strCookie []string
	)

	for _, v := range cookie {
		if v != nil && v.Value != "" || len(field) == 0 || hasStrings(field, v.Name) {
			strCookie = append(strCookie, v.Name+"="+v.Value)
		}
	}
	return strings.Join(strCookie, ";")
}
func hasStrings(ss []string, s string) bool {
	for i := 0; i < len(ss); i++ {
		if strings.EqualFold(ss[i], s) {
			return true
		}
	}
	return false
}
