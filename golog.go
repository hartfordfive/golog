package main

import (
	"flag"
	"fmt"
	"net/http"
	//"encoding/base64"
	"./lib"
	"github.com/abh/geoip"
	"github.com/fzzy/radix/redis"
	"image"
	"image/png"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Struct that contains the runtime properties including the buffered bytes to be written

const (
	pixel         string = "lib/pixel.png"
	geoip_db_base        = "http://geolite.maxmind.com/download/geoip/database/"
	geoip_db      string = "GeoIP.dat"
	geoip_db_city string = "GeoLiteCity.dat"
)

var (
	loadOnce sync.Once
	pngPixel image.Image
	geo      *geoip.GeoIP
	redisClient *redis.Client
)

type LoggerState struct {
     	MaxBuffLines      int
	BuffLines         []string
	BuffLineCount     int
	CurrLogDir        string
	CurrLogFileHandle *os.File
	CurrLogFileName   string
	LogBaseDir        string
}

var state = LoggerState{}

func loadPNG() {
	f, err := os.Open(pixel)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	m, err := png.Decode(f)
	if err != nil {
		panic(err)
	}
	pngPixel = m
}

func getInfo(ip string) (string, string, string, string, string) {

	matches := regexp.MustCompile(`([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)`).FindStringSubmatch(ip)
	if len(matches) >= 1 && geo != nil {
		record := geo.GetRecord(ip)
		if record != nil {
			return record.ContinentCode, record.CountryCode, record.CountryName, record.Region, record.City
		}
	}
	return "", "", "", "", ""
}

func loadGeoIpDb(dbName string) *geoip.GeoIP {

	// Open the GeoIP database
	geo, geoErr := geoip.Open(dbName)
	if geoErr != nil {
		fmt.Printf("Warning, could not open GeoIP database: %s\n", geoErr)
	}
	return geo
}

func getMonthAsIntString(m string) string {

	switch m {
	case "January":
		return "01"
	case "Februrary":
		return "02"
	case "March":
		return "03"
	case "April":
		return "04"
	case "May":
		return "05"
	case "June":
		return "06"
	case "July":
		return "07"
	case "August":
		return "08"
	case "September":
		return "09"
	case "October":
		return "10"
	case "November":
		return "11"
	case "December":
		return "12"
	}
	return "01"
}

func getLogfileName() string {
	y, m, d := time.Now().Date()
	return strconv.Itoa(y) + "-" + getMonthAsIntString(m.String()) + "-" + strconv.Itoa(d) + "-" + strconv.Itoa(time.Now().Hour()) + "00.txt"
}

func logHandler(res http.ResponseWriter, req *http.Request) {

	// Take the URI and parse it
	// If invalid, return tracking pixel immediately and return

	parts, err := url.Parse(req.URL.String())
	if err != nil {
		res.Header().Set("Cache-control", "public, max-age=0")
		res.Header().Set("Content-Type", "image/png")
		png.Encode(res, pngPixel)
		return
	}

	//Get the current year, month, day and hour (e.g.: YYYY-MM-DD-HHHH) to build the logfile name
	ts := int(time.Now().Unix())

	// Parse the URI
	var ln string
	params := parts.Query()

	//Log line format:  [TIMESTAMP] - [IP] - [COUNTRY] - [REGION] - [CITY] - [CATEGORY] - [ACTION] - [LABEL] - [VALUE] - [UA]

	// Extract the IP and get its related info
	matches := regexp.MustCompile(`([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)`).FindStringSubmatch(req.RemoteAddr)
	continent := ""
	countryCode := ""
	//country := ""
	region := ""
	city := ""
	ip := ""
	cid := ""
	category := ""
	action := ""
	label := ""
	value := ""

	
	if len(matches) >= 1 {

	   	// continent, countryCode, country, region, city
		continent, countryCode, _, region, city = getInfo(matches[1])
		ip = matches[1]
		currHour := strconv.Itoa(time.Now().Hour())

		t := time.Now()
		y,m,d := t.Date()
		expiryTime := time.Date(y, m, d+1, 0, 0, 0, 0, time.Local)

		if redisClient != nil {
		
		   // Increment the necessary "Country" related counters in the hashed set
		   redisRes,err := redisClient.Cmd("hexists", "country_hits_"+countryCode, currHour).Bool()

		   // If country exists in hashed set, then increment the value
		   redisErrHandler(err)
		   if redisRes == true {
		      _, err = redisClient.Cmd("hincrby", "country_hits_"+countryCode, currHour, 1).Int()
		      redisErrHandler(err)
		   } else {
		     _, err = redisClient.Cmd("hset", "country_hits_"+countryCode, currHour, 1).Int()
		     redisErrHandler(err)
		     // Set the expiry for this key to 00:00:00 tomorrow so that new data can take its place
		     if err == nil {
		     	 resExpire, err := redisClient.Cmd("expireat", "continent_hits_"+continent, expiryTime.Unix()).Int()		     	
			 if resExpire == 1 && err == nil {
			    fmt.Println("\tRedis expire successful")
			 }
		     }
		   }


		   // Now increment the necessary "Continent" related counters in the hashed set
                   redisRes,err = redisClient.Cmd("hexists", "continent_hits_"+continent, currHour).Bool()

                   // If continent exists in hashed set, then increment the value
                   redisErrHandler(err)
                   if redisRes == true {
                      _, err = redisClient.Cmd("hincrby", "continent_hits_"+continent, currHour, 1).Int()
                      redisErrHandler(err)
                   } else {
                     _, err = redisClient.Cmd("hset", "continent_hits_"+continent, currHour, 1).Int()
                     redisErrHandler(err)
		     // Set the expiry for this key to 00:00:00 tomorrow so that new data can take its place
                     if err == nil {
		     	resExpire, err := redisClient.Cmd("expireat", "continent_hits_"+continent, expiryTime.Unix()).Int()
			if resExpire == 1 && err == nil {
                            fmt.Println("\tRedis expire successful")
                        }

                     }

                   }

		}

	}

	ln += "[" + strconv.Itoa(ts) + "] ~ " + ip + " ~ " + countryCode + " ~ " + region + " ~ " + city + " ~ "
	
	 _, ok := params["cid"]
        if ok {
                cid = strings.Replace(params.Get("cid"), "~", "-", -1)
        }
	_, ok = params["category"]
	if ok {
		category = strings.Replace(params.Get("category"), "~", "-", -1)
	}
	_, ok = params["action"]
	if ok {
		action = strings.Replace(params.Get("action"), "~", "-", -1)
	}
	_, ok = params["label"]
	if ok {
		label = strings.Replace(params.Get("label"), "~", "-", -1)
	}
	_, ok = params["value"]
	if ok {
		value = strings.Replace(params.Get("value"), "~", "-", -1)
	}

	ln += cid + " ~" + category + " ~ " + action + " ~ " + label + " ~ " + value + " ~ " + req.Header.Get("User-Agent") + "\n"

	state.BuffLines = append(state.BuffLines, ln)
	state.BuffLineCount++

	// If there are 25 lines to be written, flush the buffer to the logfile
	if state.BuffLineCount >= state.MaxBuffLines {

		if getLogfileName() != state.CurrLogFileName {
			fh, _ := os.Create(strings.TrimRight(state.LogBaseDir, "/") + "/" + getLogfileName())
			state.CurrLogFileName = getLogfileName()
			state.CurrLogFileHandle = fh
			defer state.CurrLogFileHandle.Close()
		}

		totalBytes := 0
		for i := 0; i < state.BuffLineCount; i++ {
			nb, _ := state.CurrLogFileHandle.WriteString(state.BuffLines[i])
			totalBytes += nb
		}
		state.CurrLogFileHandle.Sync()
		// Empty the buffer and reset the buff line count to 0
		state.BuffLineCount = 0
		state.BuffLines = []string{}
	}

	// Finally, return the tracking pixel and exit the request.
	res.Header().Set("Cache-control", "public, max-age=0")
	res.Header().Set("Content-Type", "image/png")
	png.Encode(res, pngPixel)
	return

}


func redisErrHandler(err error) {
     if err != nil { 
     	fmt.Println("Redis error:", err)
     }
}

func main() {

	var logBaseDir,ip string
	var buffLines, port, redisDb int

	flag.StringVar(&logBaseDir, "d", "/var/log/golog/", "Base directory where log files will be written to")
	flag.StringVar(&ip, "i", "", "IP to listen on")
	flag.IntVar(&port, "p",80, "Port number to listen on")
	flag.IntVar(&buffLines, "b", 25, "Number of lines to buffer before dumping to log file")
	flag.IntVar(&redisDb, "db", 2, "Index of redis DB to use")

	flag.Parse()

	state.MaxBuffLines = buffLines

	// Load the transparent PNG pixel into memory once
	loadOnce.Do(loadPNG)

	// Ensure the GeoIP DB is available
	if tools.FileExists(geoip_db_city) == false {
		if tools.Download(geoip_db_base + geoip_db_city + ".gz") {
			fmt.Println("Download of " + geoip_db_city + " successful")
		} else {
			fmt.Println("Could not download " + geoip_db_city)
		}
	}

	geo = loadGeoIpDb(geoip_db_city)

	// Check if the specified directory exists and is writable by the current user
	if _, err := os.Stat(logBaseDir); err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(logBaseDir, 0755)
		}
		if err != nil {
			fmt.Println("Could not created directory: ", logBaseDir)
			fmt.Println("Please run process as authorized user!\n")
			os.Exit(0)
		}
	}

	fh, _ := os.Create(strings.TrimRight(logBaseDir, "/") + "/" + getLogfileName())
	state.CurrLogFileName = getLogfileName()
	state.CurrLogFileHandle = fh
	state.LogBaseDir = logBaseDir

	defer state.CurrLogFileHandle.Close()


	// Finally, load the redis instance
	c, redisErr := redis.DialTimeout("tcp", "127.0.0.1:6379", time.Duration(2)*time.Second)
	redisErrHandler(redisErr)
	redisClient = c
	defer redisClient.Close()

	// select database
	r := redisClient.Cmd("select", redisDb)
	redisErrHandler(r.Err)

	http.HandleFunc("/", logHandler)
	err := http.ListenAndServe(ip+":"+strconv.Itoa(port), nil)
	if err != nil {
	   fmt.Println("GoLog Error:", err)
	   os.Exit(0)
	}

}
