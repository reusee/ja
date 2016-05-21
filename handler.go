package ja

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"unicode"
)

type Handler struct {
	methods    map[string]*_Method
	hooks      []Hook
	methodName func(*http.Request) string
}

type Hook func(w http.ResponseWriter, req *http.Request) error

type _Method struct {
	api               reflect.Value
	reqType, respType reflect.Type
	name              string
	fn                reflect.Value
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
		m := &_Method{
			api:      apiValue,
			reqType:  methodType.In(1).Elem(),
			respType: methodType.In(2).Elem(),
			name:     method.Name,
			fn:       method.Func,
		}
		h.methods[m.name] = m
	}
	h.methodName = methodName
}

type _Response struct {
	Status string      `json:"status"`
	Result interface{} `json:"result"`
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
	fmt.Printf("%q\n", content)
	reqData := reflect.New(method.reqType)
	if err := json.Unmarshal(content, reqData.Interface()); err != nil {
		fmt.Printf("%v\n", err)
		responseStatus(w, "bad request")
		return
	}
	// call method
	respData := reflect.New(method.respType)
	callError := method.fn.Call([]reflect.Value{
		method.api, reqData, respData,
	})[0].Interface()
	if callError != nil {
		fmt.Printf("%v\n", callError)
		responseStatus(w, "call error")
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
