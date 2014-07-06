package main

import (
    "log"
    "fmt"
    "net/url"
    "net/http"
    "io"
    "io/ioutil"
    "os"
    "os/exec"
    "os/user"
    "encoding/json"
    "path/filepath"
)

func get_token(account string) (string, error) {
    auth_url := "https://accounts.google.com/o/oauth2/auth?" + url.Values{
        "response_type": {"token"},
        "client_id": {"175384822318-g58vh7hdbla895f1b869ed4h3apu5bb1.apps.googleusercontent.com"},
        "redirect_uri": {"http://localhost:8888/oauth2callback"},
        "scope": {"https://www.googleapis.com/auth/contacts.readonly email https://www.googleapis.com/auth/admin.directory.user.readonly"},
        "login_hint": {account},
    }.Encode()

    var token string
    var rerr error

    done := make(chan bool)

    http.HandleFunc("/oauth2callback", func (w http.ResponseWriter, req *http.Request) {
        io.WriteString(w, `<html><body><script>
            window.location = window.location.toString().replace('oauth2callback#', 'token?')
        </script></body></html>`)
    })

    http.HandleFunc("/token", func (w http.ResponseWriter, req *http.Request) {
        io.WriteString(w, "Done!\n")
        token = req.URL.Query().Get("access_token")
        done <- true
    })

    go http.ListenAndServe("localhost:8888", nil)

    exec.Command("xdg-open", auth_url).Start()
    fmt.Fprintln(os.Stderr, "Firefox started")

    <-done
    return token, rerr
}

func gcall(api_url string, token string) ([]byte, error) {
    resp, err := http.Get(api_url + "access_token=" + token)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    return ioutil.ReadAll(resp.Body)
}

func verify_email(account string, token string) (bool, error) {
    body, err := gcall("https://www.googleapis.com/plus/v1/people/me?", token)
    if err != nil {
        return false, err
    }
    var res struct {
        Emails []struct {
            Value string
            Type string
        }
    }
    if err := json.Unmarshal(body, &res); err != nil {
        return false, err
    }

    for _, email := range res.Emails {
        if account == email.Value {
            return true, nil
        }
    }

    fmt.Println(string(body))
    return false, nil
}

func get_cached_token(account string) (string, error) {
    usr, err := user.Current()
    if err != nil {
        return "", err
    }

    cache_dir := filepath.Join(usr.HomeDir, ".cache", "gcontacts")
    token_fname := filepath.Join(cache_dir, account)

    token, err := ioutil.ReadFile(token_fname)
    if err != nil {
        if os.IsNotExist(err) {
            stoken, err := get_token(account)
            if err == nil {
                os.MkdirAll(cache_dir, 0777)
                if err := ioutil.WriteFile(token_fname, []byte(stoken), 0777); err != nil {
                    return "", err
                }
                return stoken, err
            }
        }
    }
    return string(token), err
}

func main() {
    account := os.Args[1]
    token, err := get_cached_token(account)
    if err != nil {
        log.Fatal(err)
    }

    valid, err := verify_email(account, token)
    if err != nil {
        log.Fatal(err)
    }
    if !valid {
        log.Fatalf("Invalid email %s for token", account)
    }

    body, _ := gcall("https://www.googleapis.com/admin/directory/v1/users?domain=dreamindustries.co&", token)
    fmt.Println(string(body))
}
