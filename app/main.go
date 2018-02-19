package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/gorilla/mux"
)

var (
	MongoURL  = "db:4000/interview"
	Port      = 4260
	TTLMinute = 1
)

func main() {

	fmt.Printf("main works\n")
	ips, err := net.LookupIP("db")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Lookup for db returns the following IPs:")
	for _, ip := range ips {
		log.Printf("%v", ip)
	}

	err = MongoConnect(MongoURL)
	fmt.Printf("mongoerr: %v\nmongourl: %v | Port: %v\n", err, MongoURL, Port)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
	router := NewRouter()
	http.ListenAndServe(fmt.Sprintf(":%d", Port), router)

}

// routes

// Route is the structure of an route
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

// Routes is a list of routes
type Routes []Route

var routes = Routes{
	Route{
		"GetAllKeyValues",
		"GET",
		"/values",
		GetAllKeyValues,
	},
	Route{
		"PostKeyValues",
		"POST",
		"/values",
		PostKeyValues,
	},
	Route{
		"PatchKeyValues",
		"PATCH",
		"/values",
		PatchKeyValues,
	},
}

// api

type RetValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	// Modified time.Time `json:"modified"`
}

type PatchResponse struct {
	NotFoundKeys []string `json:"notfoundkeys"`
	UpdatedKeys  []string `json:"updatedkeys"`
}

func PatchKeyValues(w http.ResponseWriter, r *http.Request) {
	json_map := make(map[string]interface{})
	_ = json.NewDecoder(r.Body).Decode(&json_map)
	ret := PatchResponse{}
	for key, val := range json_map {
		v, _ := GetPair(key)
		duration := time.Since(v.Modified)
		if duration.Minutes() > float64(TTLMinute) {
			fmt.Printf("over TTLMinute\n")
			v.Delete()
			ret.NotFoundKeys = append(ret.NotFoundKeys, key)
		} else {
			v.Key = key
			v.Value = val.(string)
			v.Modified = time.Now()
			v.Put()
			ret.UpdatedKeys = append(ret.UpdatedKeys, key)
		}
	}

	ServeJSON(w, ret)
}

type PostKeysResponse struct {
	NewKeys []string `json:"newkeys"`
	OldKeys []string `json:"oldkeys"`
}

func PostKeyValues(w http.ResponseWriter, r *http.Request) {

	json_map := make(map[string]interface{})
	_ = json.NewDecoder(r.Body).Decode(&json_map)
	ret := PostKeysResponse{}

	for key, val := range json_map {
		fmt.Printf("k: %v val: %v\n", key, val)
		if KeyExist(key) {
			v, _ := GetPair(key)
			v.Value = val.(string)
			v.Modified = time.Now()
			v.Put()
			ret.OldKeys = append(ret.OldKeys, key)
		} else {
			v := KeyValues{}

			v.Key = key
			v.Value = val.(string)
			v.Modified = time.Now()
			v.Put()
			ret.NewKeys = append(ret.NewKeys, key)
		}
	}

	ServeJSON(w, ret)
}

func GetAllKeyValues(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("GetAllKeyValues en\n")

	keys := r.URL.Query().Get("keys")
	keyset := strings.Split(keys, ",")
	fmt.Printf("keys: %v type: %T\n", keyset, keyset)

	ret := []RetValue{}
	fmt.Printf("lenkey: %v\n", len(keyset))
	if len(keyset) > 0 && keyset[0] != "" {
		fmt.Printf("en keyset\n")
		for _, u := range keyset {
			fmt.Printf("u: %v\n", u)
			dbpair, _ := GetPair(u)
			duration := time.Since(dbpair.Modified)
			fmt.Printf("duration %v\n", duration.Minutes())
			if duration.Minutes() > float64(TTLMinute) {
				fmt.Printf("over TTLMinute\n")
				dbpair.Delete()
			} else {
				fmt.Printf("dbpair: %v\n", dbpair)
				ret = append(ret, RetValue{Key: u, Value: dbpair.Value})
				dbpair.Modified = time.Now()
				dbpair.Put()
			}
		}
	} else {
		pairs, _ := GetKeyValues()
		fmt.Printf("pairs: %v type: %T\n", pairs, pairs)
		for _, u := range pairs {
			duration := time.Since(u.Modified)
			if duration.Minutes() > float64(TTLMinute) {
				fmt.Printf("over TTLMinute\n")
				u.Delete()
			} else {
				v := RetValue{}
				v.Key = u.Key
				// v.Modified = u.Modified
				v.Value = u.Value
				ret = append(ret, v)
				u.Modified = time.Now()
				u.Put()
			}

		}

	}

	ServeJSON(w, ret)
}

// data

type KeyValues struct {
	ID       bson.ObjectId `bson:"id"`
	Key      string        `bson:"key"`
	Value    string        `bson:"value"`
	Modified time.Time     `bson:"modified"`
}

func KeyExist(key string) bool {
	db := con.session.DB("").C(keysC)
	account := KeyValues{}
	err := db.Find(bson.M{"key": key}).One(&account)
	if err != nil {
		return false
	} else if account.ID == "" {
		return false
	}
	return true
}

func (u *KeyValues) Delete() error {
	store := con.session.DB("").C(keysC)

	_ = store.Remove(bson.M{"id": u.ID})
	return nil

}

func (u *KeyValues) Put() error {
	store := con.session.DB("").C(keysC)
	if u.ID == "" {
		u.ID = bson.NewObjectId()
	}
	_, _ = store.UpsertId(u.ID, u)
	return nil

}

func GetPair(key string) (KeyValues, error) {
	store := con.session.DB("").C(keysC)
	res := KeyValues{}
	_ = store.Find(bson.M{"key": key}).One(&res)

	return res, nil
}

func GetKeyValues() ([]KeyValues, error) {
	store := con.session.DB("").C(keysC)
	res := []KeyValues{}
	_ = store.Find(nil).All(&res)

	return res, nil
}

// router

// NewRouter returns a new router
func NewRouter() *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		var handler http.Handler
		handler = route.HandlerFunc
		handler = Logger(handler, route.Name)
		router.Methods(route.Method).Path(route.Pattern).Name(route.Name).Handler(handler)
	}
	return router
}

// Logger
func Logger(inner http.Handler, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		inner.ServeHTTP(w, r)
		log.Printf(
			"%s\t%s\t%s\t%s",
			r.Method,
			r.RequestURI,
			name,
			time.Since(start),
		)
	})
}

// mongodb
const (
	keysC = "keys"
)

var con Connection

// Connection is the mongo connection struct
type Connection struct {
	session *mgo.Session
}

// Connect method returns a (Connection,err)
func MongoConnect(url string) error {
	session, err := mgo.Dial(url)
	if err != nil {
		return err
	}
	con = Connection{session: session}
	return nil
}

// Close method closes the current connection
func (c *Connection) Close() {
	c.session.Close()
}

// httpresponse

func ServeJSON(w http.ResponseWriter, data interface{}) {

	buf := &bytes.Buffer{}
	err := json.NewEncoder(buf).Encode(data)

	if err != nil {
		ServeInternalServerError(w)
	}

	w.Header().Set("Content-Type", "application/json")

	_, err = io.Copy(w, buf)
	if err != nil {
		log.Println(err)
	}
}

func ServeInternalServerError(w http.ResponseWriter) {

	w.WriteHeader(http.StatusInternalServerError)
	responseJSON := map[string]interface{}{
		"error": "Internal Server Error",
	}

	ServeJSON(w, responseJSON)
}
