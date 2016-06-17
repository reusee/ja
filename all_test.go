package ja

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

type Api struct{}

func methodName(r *http.Request) string {
	pathParts := strings.Split(r.URL.Path, "/")
	return pathParts[len(pathParts)-1]
}

func (a *Api) Ping(req *struct {
	GoodId    int64 `json:"good_id"`
	Greetings string
}, resp *struct {
	Echo string
	Num  int64
}) error {
	if req.Greetings == "foobar" {
		return fmt.Errorf("foobar")
	}
	resp.Echo = req.Greetings
	resp.Num = req.GoodId
	return nil
}

// will not register
func (a *Api) Foo()                              {}
func (a *Api) foo()                              {}
func (a *Api) Bar(req *struct{}, resp *struct{}) {}
func (a *Api) Baz(req *struct{})                 {}
func (a *Api) Quux(req *struct{}, resp *struct{}) int {
	return 0
}

func (a *Api) Bad(req *struct{}, resp *struct {
	Foo func(int) int
}) error {
	resp.Foo = func(i int) int { return i }
	return nil
}

func TestCall(t *testing.T) {
	addr := ":7899"

	c := 0
	handler := NewHandler(func(w http.ResponseWriter, r *http.Request) error {
		fmt.Printf("request %d\n", c)
		c++
		if c == 2 {
			return fmt.Errorf("error")
		}
		return nil
	})
	handler.Register(new(Api), methodName)
	if len(handler.methods) != 2 {
		t.Fatalf("wrong number of methods registered")
	}
	http.Handle("/", handler)
	go http.ListenAndServe(addr, nil)

	str := "hello, world!"

	// normal
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(struct {
		GoodId    int64 `json:"good_id"`
		Greetings string
	}{
		42,
		str,
	}); err != nil {
		t.Fatalf("request encode error: %v", err)
	}
	reqData := buf.Bytes()
	resp, err := http.Post("http://localhost"+addr+"/Ping", "application/json", bytes.NewReader(reqData))
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body error: %v", err)
	}
	var ret struct {
		Status string
		Result struct {
			Echo string
			Num  int64
		}
	}
	if err := json.Unmarshal(content, &ret); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if ret.Result.Echo != str {
		t.Fatalf("echo not match")
	}
	if ret.Result.Num != 42 {
		t.Fatalf("num not match")
	}

	// hook error
	resp, err = http.Post("http://localhost"+addr+"/Ping", "application/json", bytes.NewReader(reqData))
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()
	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body error: %v", err)
	}
	if err := json.Unmarshal(content, &ret); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if ret.Status != "error" {
		t.Fatalf("not error")
	}

	// no method
	resp, err = http.Post("http://localhost"+addr, "application/json", bytes.NewReader(reqData))
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()
	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body error: %v", err)
	}
	if err := json.Unmarshal(content, &ret); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if ret.Status != "no such method" {
		t.Fatalf("status not match")
	}

	// invalid json
	resp, err = http.Post("http://localhost"+addr+"/Ping", "application/json", bytes.NewReader([]byte("+")))
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()
	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body error: %v", err)
	}
	if err := json.Unmarshal(content, &ret); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if ret.Status != "bad request" {
		t.Fatalf("status not match")
	}

	// invalid response struct
	resp, err = http.Post("http://localhost"+addr+"/Bad", "application/json", bytes.NewReader([]byte("{}")))
	if err == nil {
		t.Fatalf("no error")
	}

	// bad call
	resp, err = http.Post("http://localhost"+addr+"/Ping", "application/json", bytes.NewReader([]byte(`
	{"Greetings": "foobar"}
	`)))
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()
	content, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body error: %v", err)
	}
	if err := json.Unmarshal(content, &ret); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if ret.Status != "call error" {
		t.Fatalf("status not match")
	}

}

func BenchmarkEcho(b *testing.B) {
	addr := ":7898"
	handler := NewHandler()
	handler.Register(new(Api), methodName)
	mux := http.NewServeMux()
	mux.Handle("/", handler)
	go http.ListenAndServe(addr, mux)

	str := "hello, world!"
	buf := new(bytes.Buffer)
	json.NewEncoder(buf).Encode(struct {
		Greetings string
	}{
		str,
	})
	reqData := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := http.Post("http://localhost"+addr+"/Ping", "application/json", bytes.NewReader(reqData))
		if err != nil {
			b.Fatalf("request error: %v", err)
		}
		defer resp.Body.Close()
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			b.Fatalf("read body error: %v", err)
		}
	}
}
