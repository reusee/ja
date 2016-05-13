package ja

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"unicode"
)

type Handler struct {
	methods map[string]*Method
	hooks   []Hook
}

type Hook func(w http.ResponseWriter, req *http.Request) error

type Method struct {
	api               reflect.Value
	reqType, respType reflect.Type
	name              string
	fn                reflect.Value
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func NewHandler(hooks ...Hook) *Handler {
	return &Handler{
		methods: make(map[string]*Method),
		hooks:   hooks,
	}
}

func (h *Handler) Register(o interface{}) {
	objType := reflect.TypeOf(o)
	nMethods := objType.NumMethod()
	apiValue := reflect.ValueOf(o)
	for i := 0; i < nMethods; i++ {
		method := objType.Method(i)
		if unicode.IsLower(rune(method.Name[0])) {
			continue
		}
		methodType := method.Type
		nArgs := methodType.NumIn()
		if nArgs != 3 {
			continue
		}
		nReturns := methodType.NumOut()
		if nReturns != 1 {
			continue
		}
		if methodType.Out(0) != errorType {
			continue
		}
		m := &Method{
			api:      apiValue,
			reqType:  methodType.In(1).Elem(),
			respType: methodType.In(2).Elem(),
			name:     method.Name,
			fn:       method.Func,
		}
		h.methods[m.name] = m
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// hooks
	for _, hook := range h.hooks {
		if err := hook(w, r); err != nil {
			http.Error(w, "Internal Server Error", 500)
			return
		}
	}
	// requested method
	pathParts := strings.Split(r.URL.Path, "/")
	what := pathParts[len(pathParts)-1]
	var method *Method
	var ok bool
	if method, ok = h.methods[what]; !ok { // no method
		http.Error(w, "Method Not Found", 405)
		return
	}
	// decode request data
	reqData := reflect.New(method.reqType)
	de := json.NewDecoder(r.Body)
	if err := de.Decode(reqData.Interface()); err != nil {
		http.Error(w, "Request Decode Error", 400)
		return
	}
	// call method
	respData := reflect.New(method.respType)
	err := method.fn.Call([]reflect.Value{
		method.api, reqData, respData,
	})[0].Interface()
	if err != nil {
		http.Error(w, "Call Error", 580)
		return
	}
	// encode response data
	en := json.NewEncoder(w)
	if err := en.Encode(respData.Interface()); err != nil {
		http.Error(w, "Response Encode Error", 581)
		return
	}
}
