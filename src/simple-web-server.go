package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type MySecrets struct {
	ConfigLocation string
	DbCon          string
	DbUser         string
	DbPassword     string
}

// func (secrets *MySecrets) ServeHTTP(w http.ResponseWriter, r *http.Request) {
// 	w.Header().Set("Content-Type", "text/html; charset=utf-8")
// 	fmt.Fprintf(w, "<body><h1>I am a GO application running inside Kubernetes.</h1> <h2>My properties are:</h2>")

// 	fmt.Fprintf(w, "<p>I read my secrets from %s</p>", secrets.configLocation)

// 	fmt.Fprintf(w, "<h2> Database connection details</h2>")
// 	fmt.Fprintf(w, "<ul><li>%s</li>", secrets.dbCon)
// 	fmt.Fprintf(w, "<li>%s</li>", secrets.dbUser)
// 	fmt.Fprintf(w, "<li>%s</li>", secrets.dbPassword)
// 	fmt.Fprintf(w, "</ul></body>")

// }

func main() {

	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}

	secrets := MySecrets{}
	secrets.readCurrentConfiguration()

	// Kubernetes check if app is ok
	http.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "up")
	})

	// Kubernetes check if app can serve requests
	http.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "yes")
	})

	http.HandleFunc("/", secrets.serveFiles)

	fmt.Printf("My Secret App is listening now at port %s\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func (secrets *MySecrets) readCurrentConfiguration() {
	viper.SetDefault("db_con", "mysql.example.com:3306")
	viper.SetDefault("db_user", "demoUser")
	viper.SetDefault("db_password", "demoPassword")

	viper.SetConfigName("credentials")
	viper.SetConfigType("properties")

	//Development mode
	viper.AddConfigPath(".")

	//Proper configuration in non-development mode
	viper.AddConfigPath("/secrets/")

	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {             // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %w", err))
	}

	//Reload configuration when file changes
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
		secrets.reloadSettings()

	})

	secrets.reloadSettings()

	viper.WatchConfig()

}

func (secrets *MySecrets) reloadSettings() {

	secrets.ConfigLocation = viper.ConfigFileUsed()
	fmt.Printf("Reading configuration from %s\n", secrets.ConfigLocation)

	secrets.DbCon = unQuoteIfNeeded(viper.GetString("db_con"))
	secrets.DbUser = unQuoteIfNeeded(viper.GetString("db_user"))
	secrets.DbPassword = unQuoteIfNeeded(viper.GetString("db_password"))

	fmt.Printf("Connection string is %s\n", secrets.DbCon)
	fmt.Printf("Username is %s\n", secrets.DbUser)
	fmt.Printf("Password is %s\n", secrets.DbPassword)

}

func unQuoteIfNeeded(input string) string {
	result := ""
	if strings.HasPrefix(input, "\"") {
		result, _ = strconv.Unquote(input)
	}
	return result
}

func (secrets *MySecrets) home(w http.ResponseWriter, r *http.Request) {

	// microserviceStatus.findBackendVersion()
	// microserviceStatus.findStatus()

	t, err := template.ParseFiles("./static/index.html")
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Error parsing template: %v", err)
		return
	}
	err = t.Execute(w, secrets)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("Error executing template: %v", err)
		return
	}
}

func (secrets *MySecrets) serveFiles(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	p := "." + upath
	// fmt.Printf("Path:Upath is %s:%s\n", p, upath)

	if p == "./" {
		secrets.home(w, r)
		return
	} else {
		p = filepath.Join("./static/", path.Clean(upath))
	}

	http.ServeFile(w, r, p)
}
