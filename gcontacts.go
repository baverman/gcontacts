package main

import (
    "log"
    "fmt"
    "os"
    "os/exec"
    "os/user"
    "time"
    "net/url"
    "net/http"
    "io"
    "io/ioutil"
    "encoding/json"
    "path/filepath"
)

func get_token(account string) string {
    auth_url := "https://accounts.google.com/o/oauth2/auth?" + url.Values{
        "response_type": {"token"},
        "client_id": {"763998006187-v0be2qs7s59f5s948mbtvs4cs3sfbn12.apps.googleusercontent.com"},
        "redirect_uri": {"http://localhost:8888/oauth2callback"},
        "scope": {"https://www.googleapis.com/auth/contacts.readonly email"},
        "login_hint": {account},
    }.Encode()

    var token string
    done := make(chan bool)

    http.HandleFunc("/oauth2callback", func (w http.ResponseWriter, req *http.Request) {
        io.WriteString(w, `<html><body><script>
            window.location = window.location.toString().replace('oauth2callback#', 'token?')
        </script></body></html>`)
    })

    http.HandleFunc("/token", func (w http.ResponseWriter, req *http.Request) {
        fmt.Println(req.URL)
        io.WriteString(w, "Done!\n")
        token = req.URL.Query().Get("access_token")
        done <- true
    })

    go http.ListenAndServe("localhost:8888", nil)

    exec.Command("xdg-open", auth_url).Start()
    fmt.Fprintln(os.Stderr, "Firefox started")

    <-done
    return token
}

func gcall(api_url string, token string) []byte {
    resp, err := http.Get(api_url + "access_token=" + token)
    if err != nil {
        panic(err)
    }

    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        panic(err)
    }
    return body
}

func valid_email(account string, token string) bool {
    body := gcall("https://www.googleapis.com/plus/v1/people/me?", token)

    var res struct {
        Emails []struct {
            Value string
            Type string
        }
    }
    if err := json.Unmarshal(body, &res); err != nil {
        panic(err)
    }

    for _, email := range res.Emails {
        if account == email.Value {
            return true
        }
    }

    fmt.Println(string(body))
    return false
}

func get_cached_token(account string) string {
    usr, err := user.Current()
    if err != nil {
        panic(err)
    }

    cache_dir := filepath.Join(usr.HomeDir, ".cache", "gcontacts")
    token_fname := filepath.Join(cache_dir, account)

    token, err := ioutil.ReadFile(token_fname)
    if err != nil {
        if os.IsNotExist(err) {
            stoken := get_token(account)
            os.MkdirAll(cache_dir, 0777)
            if err := ioutil.WriteFile(token_fname, []byte(stoken), 0600); err != nil {
                panic(err)
            }
            return stoken
        } else {
            panic(err)
        }
    }

    info, err := os.Stat(token_fname)
    if err != nil {
        panic(err)
    }
    if info.ModTime().Before(time.Now().Add(-50 * time.Minute)) {
        stoken := get_token(account)
        if err := ioutil.WriteFile(token_fname, []byte(stoken), 0600); err != nil {
            panic(err)
        }
        return stoken
    }

    return string(token)
}

func main() {
    defer func() {
        if err := recover(); err != nil {
            log.Fatal(err)
        }
    }()

    account := os.Args[1]
    token := get_cached_token(account)

    if !valid_email(account, token) {
        log.Fatalf("Invalid email %s for token", account)
    }

    // body, _ := gcall("https://www.googleapis.com/admin/directory/v1/users?domain=dreamindustries.co&", token)
    // body, _ := gcall("https://www.google.com/m8/feeds/groups/default/full/27?alt=json&", token)
    body := gcall("https://www.google.com/m8/feeds/contacts/default/full?alt=json&", token)
    fmt.Println(string(body))
}
