package ja

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
)

type Api struct{}

func (a *Api) Ping(req *struct {
	Greetings string
}, resp *struct {
	Echo string
}) error {
	resp.Echo = req.Greetings
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
	handler.Register(new(Api))
	if len(handler.methods) != 2 {
		t.Fatalf("wrong number of methods registered")
	}
	http.Handle("/", handler)
	go http.ListenAndServe(":7899", nil)

	str := "hello, world!"

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(struct {
		Greetings string
	}{
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
		t.Fatalf("read body error: %v")
	}
	var ret struct {
		Echo string
	}
	if err := json.Unmarshal(content, &ret); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if ret.Echo != str {
		t.Fatalf("echo not match")
	}

	// hook error
	resp, err = http.Post("http://localhost"+addr+"/Ping", "application/json", bytes.NewReader(reqData))
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Fatalf("not 500")
	}

	// no method
	resp, err = http.Post("http://localhost"+addr, "application/json", bytes.NewReader(reqData))
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("not 404")
	}

	// invalid json
	resp, err = http.Post("http://localhost"+addr+"/Ping", "application/json", bytes.NewReader([]byte("+")))
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Fatalf("not 500")
	}

	// invalid response struct
	resp, err = http.Post("http://localhost"+addr+"/Bad", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Fatalf("not 500")
	}

}
