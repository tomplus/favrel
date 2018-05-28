package main

import "flag"
import "fmt"
import "os"
import "net/http"
import "log"
import "io/ioutil"
import "io"
import "bytes"
import "encoding/json"
import "time"
import "strconv"

const dbFile = "favrel.data"
const apiEndpoint = "https://api.github.com"

func apiQuery(uri string) (int, []byte) {

	resp, err := http.Get(apiEndpoint + uri)
	if err != nil {
		log.Fatalln(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	if resp.StatusCode == 403 && resp.Header["X-Ratelimit-Remaining"][0] == "0" {
		rateLimitReset, _ := strconv.ParseInt(resp.Header["X-Ratelimit-Reset"][0], 0, 64)
		reset := rateLimitReset - time.Now().Unix()
		log.Fatalln("rate limiter, you have to wait", reset/60, "minutes")
	}

	return resp.StatusCode, body
}

func getStarredProjects(account string) []string {

	fmt.Printf("Get starred projects by %s ...\n", account)
	_, body := apiQuery(fmt.Sprintf("/users/%s/starred", account))

	var f interface{}
	json.Unmarshal(body, &f)
	m := f.([]interface{})
	var ret []string
	cnt := 0
	for _, k := range m {
		km := k.(map[string]interface{})
		ret = append(ret, km["full_name"].(string))
		cnt++
	}

	fmt.Printf("found %d starred project\n", cnt)
	return ret
}

func getLatestRelease(repo string, expected string) string {

	code, body := apiQuery(fmt.Sprintf("/repos/%s/releases/latest", repo))
	if code == 404 {
		fmt.Printf("Project %s doesn't have releases\n", repo)
		return "no-releases"
	}

	var f interface{}
	json.Unmarshal(body, &f)
	m := f.(map[string]interface{})

	if m["tag_name"] == expected {
		fmt.Printf("Latest release for %s is %s - no changes\n", repo, m["tag_name"])
	} else {
		fmt.Printf("Latest release for %s is %s - previous %s\n", repo, m["tag_name"], expected)
		fmt.Println(m["html_url"])
		fmt.Println("------------------")
		fmt.Println(m["body"])
		fmt.Println("------------------")
	}

	return m["tag_name"].(string)
}

func saveData(data map[string]string) {

	f, err := os.Create(dbFile)
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()

	r, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		log.Fatalln(err)
	}

	br := bytes.NewReader(r)
	_, err = io.Copy(f, br)
	if err != nil {
		log.Fatalln(err)
	}
}

func loadData() map[string]string {

	f, err := os.Open(dbFile)
	if os.IsNotExist(err) {
		return make(map[string]string)
	} else if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()

	var v map[string]string
	err = json.NewDecoder(f).Decode(&v)
	if err != nil {
		log.Fatalln(err)
	}

	return v
}

func main() {

	githubAccountPtr := flag.String("githab-account", "tomplus", "github account")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Print latest version of your favorite projects\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	data := loadData()
	projects := getStarredProjects(*githubAccountPtr)

	for _, name := range projects {
		data_rel, ok := data[name]
		if !ok {
			data_rel = ""
		}
		rel := getLatestRelease(name, data_rel)
		data[name] = rel
	}

	saveData(data)
}
