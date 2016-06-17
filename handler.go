package ja

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"sync"
	"unicode"
)

type Handler struct {
	methods    map[string]*_Method
	hooks      []Hook
	methodName func(*http.Request) string
}

type ErrorStatus string

func (e ErrorStatus) Error() string {
	return string(e)
}

var _ error = ErrorStatus("")

type Hook func(w http.ResponseWriter, req *http.Request) error

type _Method struct {
	api               reflect.Value
	reqType, respType reflect.Type
	name              string
	fn                reflect.Value
	nArgs             int
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func NewHandler(hooks ...Hook) *Handler {
	return &Handler{
		methods: make(map[string]*_Method),
		hooks:   hooks,
	}
}

func (h *Handler) Register(o interface{}, methodName func(r *http.Request) string) {
	objType := reflect.TypeOf(o)
	nMethods := objType.NumMethod()
	apiValue := reflect.ValueOf(o)
	for i := 0; i < nMethods; i++ {
		method := objType.Method(i)
		if unicode.IsLower(rune(method.Name[0])) {
			continue
		}
		methodType := method.Type
		nReturns := methodType.NumOut()
		if nReturns != 1 {
			continue
		}
		if methodType.Out(0) != errorType {
			continue
		}
		nArgs := methodType.NumIn()
		if nArgs == 3 {
		} else if nArgs == 4 {
		} else {
			continue
		}
		m := &_Method{
			api:      apiValue,
			reqType:  methodType.In(1).Elem(),
			respType: methodType.In(2).Elem(),
			name:     method.Name,
			fn:       method.Func,
			nArgs:    nArgs,
		}
		h.methods[m.name] = m
	}
	h.methodName = methodName
}

type _Response struct {
	Status string      `json:"status"`
	Result interface{} `json:"result"`
}

type CallInfo struct {
	Method string
	Args   interface{}
	Raw    []byte
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// hooks
	for _, hook := range h.hooks {
		if err := hook(w, r); err != nil {
			responseStatus(w, err.Error())
			return
		}
	}

	// requested method
	what := h.methodName(r)
	var method *_Method
	var ok bool
	if method, ok = h.methods[what]; !ok { // no method
		responseStatus(w, "no such method")
		return
	}

	// decode request data
	content, err := ioutil.ReadAll(r.Body)
	if err != nil {
		responseStatus(w, "bad request body")
		return
	}
	reqData := reflect.New(method.reqType)
	if err := json.Unmarshal(content, reqData.Interface()); err != nil {
		fmt.Printf("%v\n", err)
		responseStatus(w, "bad request")
		return
	}

	// set context
	*r = *r.WithContext(context.WithValue(r.Context(),
		"call_info", CallInfo{
			Method: what,
			Args:   reqData.Interface(),
			Raw:    content,
		}))

	// call method
	respData := reflect.New(method.respType)
	args := []reflect.Value{
		method.api, reqData, respData,
	}
	if method.nArgs == 4 {
		args = append(args, reflect.ValueOf(r))
	}
	callError := method.fn.Call(args)[0].Interface()
	if callError != nil {
		fmt.Printf("%v\n", callError)
		switch e := callError.(type) {
		case ErrorStatus:
			responseStatus(w, e.Error())
		default:
			responseStatus(w, "call error")
		}
		return
	}

	// encode response data
	if err := json.NewEncoder(w).Encode(_Response{
		Status: "ok",
		Result: respData.Interface(),
	}); err != nil {
		panic(err)
	}

}

func responseStatus(w http.ResponseWriter, status string) {
	json.NewEncoder(w).Encode(_Response{
		Status: status,
	})
}
