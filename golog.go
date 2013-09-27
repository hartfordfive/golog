package main

import (
	"flag"
	"fmt"
	"net/http"
	"./lib"
	"github.com/abh/geoip"
	"github.com/fzzy/radix/redis"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"encoding/json"
	"encoding/base64"
	"io"
)

// Struct that contains the runtime properties including the buffered bytes to be written

const (
	PIXEL         string = "lib/pixel.png"
	PNGPX_B64     string = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGP6zwAAAgcBApocMXEAAAAASUVORK5CYII="
	geoip_db_base string = "http://geolite.maxmind.com/download/geoip/database/"
	geoip_db      string = "GeoIP.dat"
	geoip_db_city string = "GeoLiteCity.dat"
	DEBUG	      bool   = true
	VERSION_MAJOR  int    = 0
	VERSION_MINOR  int    = 1
	VERSION_PATCH  int    = 0
	VERSION_SUFFIX string = "beta"
)

var (
	loadOnce sync.Once
	pngPixel []byte
	geo      *geoip.GeoIP
	redisClient *redis.Client = nil
)

type LoggerState struct {
     	MaxBuffLines      int
	BuffLines         []string
	BuffLineCount     int
	CurrLogDir        string
	CurrLogFileHandle *os.File
	CurrLogFileName   string
	LogBaseDir        string
	CookieDomain	  string
	Config		  map[string]string
}


var state = LoggerState{Config: make(map[string]string)}

type StatsHandler struct{}
type StatsDeviceHandler struct{}
type LogHandler struct{}

func getVersion() string {
     return strconv.Itoa(VERSION_MAJOR)+"."+strconv.Itoa(VERSION_MINOR)+"."+strconv.Itoa(VERSION_PATCH)+"-"+VERSION_SUFFIX
}

func loadPNG(){
     pngPixel,_ = base64.StdEncoding.DecodeString(PNGPX_B64)
}

func getInfo(ip string) (string, string, string, string) {

	matches := regexp.MustCompile(`([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)`).FindStringSubmatch(ip)
	if len(matches) >= 1 && geo != nil {
		record := geo.GetRecord(ip)
		if record != nil {
			return record.ContinentCode, record.CountryCode, record.CountryName, record.City
		}
	}
	return "", "", "", ""
}

func loadGeoIpDb(dbName string) *geoip.GeoIP {

	// Open the GeoIP database
	geo, geoErr := geoip.Open(dbName)
	if geoErr != nil {
		fmt.Printf("Warning, could not open GeoIP database: %s\n", geoErr)
	}
	return geo
}

func writeToFile(filePath string, dataToDump string) int{
      fh, _ := os.Create(filePath)
      defer fh.Close()
      nb,_ := fh.WriteString(string(dataToDump))
      fh.Sync()
      if DEBUG { fmt.Println("Wrote "+strconv.Itoa(nb)+" bytes to "+filePath) }
      return nb
}


func getLogfileName() string {
	y, m, d := time.Now().Date()
	return strconv.Itoa(y) + "-" + tools.GetMonthAsIntString(m.String()) + "-" + strconv.Itoa(d) + "-" + strconv.Itoa(time.Now().Hour()) + "00.txt"
}


func getGeoLocationStats(resList []string) map[string]map[string]map[string]int{

     returnData := map[string]map[string]map[string]int {
                "country_hits": map[string]map[string]int{},
                "continent_hits": map[string]map[string]int{},
     }

     // Itterate over each key, and get it's data
     redisClient := getRedisConnection()

     for i := 0; i < len(resList); i++ {
         parts := strings.Split(resList[i],":")
         returnData[parts[0]][parts[1]] = map[string]int{}
         resList3,_ := redisClient.Cmd("ZRANGE", resList[i], 0 , -1, "WITHSCORES").List()
         // Initialize the map at this index and itterate over the zrange results to populate the return map
         for j := 0; j < len(resList3); j++ {
             val,_ := strconv.Atoi(resList3[j+1])
             returnData[parts[0]][parts[1]][resList3[j]] =  val
             j++
         }
     }

     return returnData
}

func getDeviceStats() map[string]map[string]int{

     returnData := map[string]map[string]int {
                "platform": map[string]int{},
		"os_version": map[string]int{},
                "browser": map[string]int{},
		"rendering_engine": map[string]int{},
		"model": map[string]int{},
     }


     redisClient := getRedisConnection()

     // Itterate over each key, and get it's data  ** Try pipelining these redis connections ***
     for k,_ := range returnData {

         returnData[k] = map[string]int{}
	 if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] ZRANGE device_stats:"+k+":"+tools.YmdToString() + " 0 -1 WITHSCORES") }

         resList3,_ := redisClient.Cmd("ZRANGE", "device_stats:"+k+":"+tools.YmdToString(), 0 , -1, "WITHSCORES").List()	 

         // Initialize the map at this index and itterate over the zrange results to populate the return map
         for j := 0; j < len(resList3); j++ {
             val,_ := strconv.Atoi(resList3[j+1])
             returnData[k][resList3[j]] =  val
             j++
         }
     }

     return returnData

}


func getRedisConnection() *redis.Client{

     // Finally, load the redis instance
     rc, redisErr := redis.DialTimeout("tcp", state.Config["redisIp"]+":"+state.Config["redisPort"], time.Duration(2)*time.Second)
     redisErrHandler(redisErr, "["+tools.DateStampAsString()+"] 1 - tcp connect")
     // Select the desired DB
     r := rc.Cmd("select", state.Config["redisDb"])
     redisErrHandler(r.Err, "["+tools.DateStampAsString()+"] Redis op error: select "+state.Config["redisDb"])
     return rc
}


func (lh *LogHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {


	// Take the URI and parse it
	// If invalid, return tracking pixel immediately and return

	parts, err := url.Parse(req.URL.String())
	if err != nil || req.URL.Path != "/" {
	   res.WriteHeader(http.StatusNotFound)
	   res.Header().Set("Cache-control", "public, max-age=0")
	   res.Header().Set("Content-Type", "text/html")
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
	city := ""
	ip := ""
	cid := ""
	category := ""
	action := ""
	label := ""
	value := ""
	udid := ""
	
	if len(matches) >= 1 {

		continent, countryCode, _, city = getInfo(matches[1])
		ip = matches[1]
		currHour := strconv.Itoa(time.Now().Hour())

		t := time.Now()
		y,m,d := t.Date()
		expiryTime := time.Date(y, m, d+1, 0, 0, 0, 0, time.Local)

		redisClient := getRedisConnection()

		if redisClient != nil {
				
		   // First check if the redis pool for the "golog_stats_available" object. If not present, then reset
		   // all stats and set this object once again to expiry tomorrow at 00h:00m:00s
		   statsOkUntil, err := redisClient.Cmd("TTL", "golog_stats_available").Int()
		   
		   if DEBUG {
                           fmt.Println("["+tools.DateStampAsString()+"] Redis command: TTL golog_stats_available (Response: "+strconv.Itoa(statsOkUntil)+")")
                   }

                   if err == nil  && statsOkUntil < 1 {
		   
			if DEBUG {
			   fmt.Println("["+tools.DateStampAsString()+"] NOTICE: Reseting all keys to restart logging period.")
		   	}

			 // Get all the keys to be deleted			 
			 /*
		      	 tmpResKeys1, _ := redisClient.Cmd("KEYS", "continent_hits:*").List()
			 tmpResKeys2, _ := redisClient.Cmd("KEYS", "country_hits:*").List()			 			 			 			 
			 resKeys := tools.JoinLists(tmpResKeys1,tmpResKeys2)			  
			 */

			 redisClient.Append("KEYS", "continent_hits:*")
			 redisClient.Append("KEYS", "country_hits:*")
			 r1,_ := redisClient.GetReply().List()
			 r2,_ := redisClient.GetReply().List()
			 resKeys := tools.JoinLists(r1,r2)			 


			 // Export the data in a json format and write it to a log file
			 dataToDump,_ := json.Marshal(getGeoLocationStats(resKeys))
	
			  if DEBUG {
                           fmt.Println("["+tools.DateStampAsString()+"] NOTICE: Writing geo location stats from previous day to JSON file in "+state.Config["logBaseDir"])
                        }


        		 _ = writeToFile(state.Config["logBaseDir"] + "_daily_geo_stats-" + strconv.Itoa(y) + "-" + strconv.Itoa(int(m)) + "-" + strconv.Itoa(d)+".json", string(dataToDump))

			 // Now dump the device stats		
			  if DEBUG {
                           fmt.Println("["+tools.DateStampAsString()+"] NOTICE: Writing device stats from previous day to JSON file in "+state.Config["logBaseDir"])
                        }

                         dataToDump,_ = json.Marshal(getDeviceStats())
			 _ = writeToFile(state.Config["logBaseDir"] + "_daily_device_stats-" + strconv.Itoa(y) + "-" + strconv.Itoa(int(m)) + "-" + strconv.Itoa(d) + ".json", string(dataToDump))
			 

			 // Now delete all keys in hashed set and			
			 // set the "golog_stats_available" key again with the proper expiry time
			 
			 for i := 0; i < len(resKeys); i++ {
			     redisClient.Append("ZREMRANGEBYRANK", resKeys[i], 0, -1)
			 }
			 redisClient.Append("SET", "golog_stats_available", 1)
			 redisClient.Append("EXPIREAT", "golog_stats_available", expiryTime.Unix())			 
			 redisClient.GetReply()		

			 if DEBUG {
			    fmt.Println("["+tools.DateStampAsString()+"] NOTICE: All related keys from yesterday have been reset.")
			 }
                   }

		   
		   // Increment the necessary "Country" and "Continent" hit related counters in the hashed set
		   for _,keyPrefix := range []string{"continent_hits", "country_hits"} {		       
			
			member := ""
			if keyPrefix == "continent_hits" {
			   member = continent
			}  else {
			   member = countryCode
			}

			/*
		   	if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis Operation [ZSCORE  "+keyPrefix+":"+currHour+" "+member+"]") }
		   	redisRes,err := redisClient.Cmd("ZSCORE", keyPrefix+":"+currHour, member).Int()
		   	redisErrHandler(err, "[ZSCORE  "+keyPrefix+":"+currHour+" "+member+"]")
			*/
					   
		   	//if string(redisRes) != "<nil>" {
		      	   if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis Operation [ZINCRBY "+keyPrefix+":"+currHour+" 1 "+member+"]") }
		      	   redisClient.Append("ZINCRBY", keyPrefix+":"+currHour , 1, member)
		      	   //redisErrHandler(err, "["+tools.DateStampAsString()+"] [ZINCRBY "+keyPrefix+":"+currHour+" 1 "+member+"]")
		   	/*
			} else {		     	
			   if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis Operation [ZADD "+keyPrefix+":"+currHour+" 1 "+member+"]")  }
		     	   _, err = redisClient.Cmd("ZADD", keyPrefix+":"+currHour, 1, member).Int()
		     	   redisErrHandler(err, "["+tools.DateStampAsString()+"] [ZADD "+keyPrefix+":"+currHour+" 1 "+member+"]")
		   	}
			*/
		   }


		   // ************  Now create the stats related to Device user agents *******************		
		   deviceDetails := tools.GetUserAgentDetails(req.Header.Get("User-Agent"))
		   if DEBUG { fmt.Println("[" + tools.DateStampAsString() + "]", deviceDetails) }
 
		   if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis Operation [ZINCRBY device_stats:platform:"+tools.YmdToString() + " 1 " + deviceDetails["platform"] + "]")  }
                   redisClient.Append("ZINCRBY", "device_stats:platform:"+tools.YmdToString(), 1, deviceDetails["platform"])
		   
		   if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis Operation [ZINCRBY device_stats:os_version:"+tools.YmdToString() + " 1 " + deviceDetails["platform"]+" - "+deviceDetails["os_version"]+"]")  }
                   redisClient.Append("ZINCRBY", "device_stats:os_version:"+tools.YmdToString(), 1, deviceDetails["platform"]+" - "+deviceDetails["os_version"])
		   
		   if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis Operation [ZINCRBY device_stats:rendering_engine:"+tools.YmdToString() + " 1 " + deviceDetails["rendering_engine"]+"]")  }
                   redisClient.Append("ZINCRBY", "device_stats:rendering_engine:"+tools.YmdToString(), 1, deviceDetails["rendering_engine"])

		   if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis Operation [ZINCRBY device_stats:browser:"+tools.YmdToString() + " 1 " + deviceDetails["browser"]+"]")  }
                   redisClient.Append("ZINCRBY", "device_stats:browser:"+tools.YmdToString(), 1, deviceDetails["browser"])

		   // Finally flush the buffer of operations to the redis server
		   redisClient.GetReply()		   		   		   

		   // Now close the redis connection
		   redisClient.Close()

		} // END of "redisClient != nil" condition

	}

	ln += "[" + strconv.Itoa(ts) + "] ~ " + ip + " ~ " + countryCode + " ~ " + city + " ~ "
	
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


	// If the _golog_uuid cookie is not set, then create the uuid and set it
        cookie := req.Header.Get("Cookie")
        if cookie != "" {
                  cookies := strings.Split(cookie, "; ")
                  for i := 0; i < len(cookies); i++ {
                      parts := strings.Split(cookies[i], "=")
                      if parts[0] == "udid" {
                         udid = parts[1]
                         break
                      }
                  }
                  // If the cookie isn't found, then generate a udid and then send the cookie
                  if udid == "" {
                      y,m,d := time.Now().Date()
                      expiryTime := time.Date(y, m, d+365, 0, 0, 0, 0, time.UTC)
                      res.Header().Set("Set-Cookie", "udid="+tools.GetUDID()+"; Domain="+state.CookieDomain+"; Path=/; Expires="+expiryTime.Format(time.RFC1123))
                  }
        }

	ln += cid + " ~ " + udid + " ~ " + category + " ~ " + action + " ~ " + label + " ~ " + value + " ~ " + req.Header.Get("User-Agent") + "\n"

	state.BuffLines = append(state.BuffLines, ln)
	state.BuffLineCount++

	// If there are 25 lines to be written, flush the buffer to the logfile
	
	if bl,_ := strconv.Atoi(state.Config["buffLines"]); state.BuffLineCount >= bl {

	   	if DEBUG { fmt.Println("Writting buffer to disk. ("+ strconv.Itoa(bl) +" lines total)" ) }

		if getLogfileName() != state.CurrLogFileName {
		        if DEBUG { fmt.Println("\t Updated Filename:", strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName()) }
			fh, _ := os.Create(strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName())
			state.CurrLogFileName = strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName()
			state.CurrLogFileHandle = fh
			defer state.CurrLogFileHandle.Close()
		}

		totalBytes := 0
		for i := 0; i < state.BuffLineCount; i++ {
			nb, _ := state.CurrLogFileHandle.WriteString(state.BuffLines[i])
			totalBytes += nb
		}
		state.CurrLogFileHandle.Sync()
		if DEBUG { fmt.Println("\t Wrote to:", state.CurrLogFileName) }
		// Empty the buffer and reset the buff line count to 0
		state.BuffLineCount = 0
		state.BuffLines = []string{}
	}

	// Finally, return the tracking pixel and exit the request.
	res.Header().Set("Cache-control", "public, max-age=0")
	res.Header().Set("Content-Type", "image/png")	
	output,_ := base64.StdEncoding.DecodeString(PNGPX_B64)
	io.WriteString(res, string(output))

	return

}


func (sh *StatsHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
         
      if req.URL.Path != "/stats" && req.URL.Path != "/statsdevices" {
      	  res.WriteHeader(http.StatusNotFound)
	  res.Header().Set("Cache-control", "public, max-age=0")
     	  res.Header().Set("Content-Type", "text/html")          
	  fmt.Fprintf(res, "Invalid path")
      	  return 
      }


      if req.URL.Path == "/stats" {

      	 redisClient := getRedisConnection()

            // Get the list of all keys to fetch
	    if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis CMD: KEYS country_hits*") }
	    if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis CMD: KEYS continent_hits*") }
      	    resList1,_ := redisClient.Cmd("keys", "country_hits*").List()	   
	    resList2,_ := redisClient.Cmd("keys", "continent_hits*").List()
	    resList := tools.JoinLists(resList1,resList2)
	    
     	    // Initalize the array with 24 indexes and for each key in the restList, populate the map
     	    data1,err1 := json.Marshal(getGeoLocationStats(resList))
     	    res.Header().Set("Cache-control", "public, max-age=0")
     	    res.Header().Set("Content-Type", "application/json")
     	    if err1 == nil {
               fmt.Fprintf(res, string(data1))
     	    } else {
       	      fmt.Fprintf(res, "{\"status\": \"error\"}")
     	    }

	    redisClient.Close()

	} else if req.URL.Path == "/statsdevices" {

	  redisClient := getRedisConnection()

	     /*
	     t := time.Now()
     	     y,m,d := t.Date()	    
	     month := ""
	     if int(m) < 10 {
	     	     month = "0"+strconv.Itoa(int(m))
 	     } else {
	       	    month = strconv.Itoa(int(m))
	     }
	    
	     if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis CMD: KEYS device_stats:*:" + strconv.Itoa(y) + month + strconv.Itoa((d-1)) ) }
     	     resKeys, _ := redisClient.Cmd("KEYS", "device_stats:*:" + strconv.Itoa(y) + month + strconv.Itoa((d-1)) ).List()
	     */

	     
     	     data,err := json.Marshal(getDeviceStats())
     	     res.Header().Set("Cache-control", "public, max-age=0")
     	     res.Header().Set("Content-Type", "application/json")
     	    if err == nil {
               fmt.Fprintf(res, string(data))
	    } else {
               fmt.Fprintf(res, "{\"status\": \"error\"}")
       	    }

	    redisClient.Close()

	}


	return
}



func redisErrHandler(err error, stamp string) {
     if err != nil { 
     	fmt.Println(stamp + " Redis error:", err)
     }
}


func loadConfig(filePath string) map[string]string{
     
     params := tools.ParseConfigFile(filePath)

     /*
	 Now verify that all necessary parameters are present, otherwise return
	 an error and exit
     */

     _,ok := params["log_base_dir"] 
     if ok != true {
     	fmt.Println("["+tools.DateStampAsString()+"] Config Error: log directory not specified!\n")
        os.Exit(0)
     } else {
       state.Config["logBaseDir"] = strings.TrimRight(params["log_base_dir"], "/") + "/"
     }     	 

     _,ok = params["server_ip"]
     if ok != true {
        fmt.Println("["+tools.DateStampAsString()+"] Config Error: server IP not specified!\n")
        os.Exit(0)
     } else {
       state.Config["ip"] = params["server_ip"]
     }

     _,ok = params["server_port"]
     if ok != true {
        fmt.Println("["+tools.DateStampAsString()+"] Config Error: server port not specified!\n")
        os.Exit(0)
     } else {
       state.Config["port"] = params["server_port"]
     }

     _,ok = params["num_buff_lines"]
     if ok != true {
        state.MaxBuffLines = 25
	state.Config["buffLines"] = "25"
     } else {
       bl,_ := strconv.Atoi(params["num_buff_lines"])
       state.MaxBuffLines = bl
       state.Config["buffLines"] = params["num_buff_lines"]
     }


     _,ok = params["redis_ip"]
     if ok != true {
        state.Config["redisIp"] = "127.0.0.1"
     } else {
       state.Config["redisIp"] = params["redis_ip"]
     }

     _,ok = params["redis_port"]
     if ok != true {
        state.Config["redisPort"] = "6379"
     } else {
       state.Config["redisPort"] = params["redis_port"]
     }

     _,ok = params["redis_db_index"]
     if ok != true {
        state.Config["redisDb"] = "2"
     } else {
       state.Config["redisDb"] = params["redis_db_index"]
     }


     _,ok = params["flush_redis_db"]
     if ok != true {
        state.Config["flushRedis"] = "1"
     } else {
       state.Config["flushRedis"] = params["flush_redis_db"]
     }

     _,ok = params["cookie_domain"]
     if ok != true {
        state.Config["cDomain"] = ""
     } else {
       state.Config["cDomain"] = params["cookie_domain"]
     }

     _,ok = params["reporting_server_active"]
     if ok != true {
        state.Config["reportingActive"] = "0"
     } else {
       state.Config["reportingActive"] = params["reporting_server_active"]
     }

     if state.Config["reportingActive"] == "1" {
     	_,ok = params["reporting_server_ip"]
	if ok != true {
           state.Config["reportingIp"] = ""
     	} else {
       	  state.Config["reportingIp"] = params["reporting_server_ip"]
     	}
     	_,ok = params["reporting_server_port"]
     	if ok != true {
           state.Config["reportingPort"] = ""
     	} else {
       	  state.Config["reportingPort"] = params["reporting_server_port"]
     	}
     }

    
    return params
}


func main() {

     	if len(os.Args) == 2 && os.Args[1] == "-version" {
           fmt.Println("GoLog - Version "+getVersion()+"\n")
           os.Exit(0)
     	}


	var logBaseDir, ip, cDomain, config, reportingIp, flushRedisDb string
	var buffLines, port, redisDb, reportingActive, reportingPort int
	
	flag.StringVar(&logBaseDir, "d", "/var/log/golog/", "Base directory where log files will be written to")
	flag.StringVar(&ip, "i", "0.0.0.0", "IP to listen on")
	flag.IntVar(&port, "p", 80, "Port number to listen on")
	flag.IntVar(&buffLines, "b", 25, "Number of lines to buffer before dumping to log file")
	flag.IntVar(&redisDb, "db", 2, "Index of redis DB to use")
	flag.StringVar(&flushRedisDb, "flushredis", "0", "Option to flush redis db on startup")
	flag.StringVar(&cDomain, "domain", "", "Domain on which to set the udid cookie on.")
	flag.IntVar(&reportingActive, "stats", 0, "Enable status reporting on [IP]:[PORT]")
	flag.StringVar(&reportingIp, "ri", "0.0.0.0", "IP to listen on for status reporting")
        flag.IntVar(&reportingPort, "rp", 80, "Port number to listen on for status reporting")	
	flag.StringVar(&config, "conf", "", "Config file to be used")

	flag.Parse()


	/*  If a config file is specified and exists, then parse it and use it,
	    otherwise just use the command-line flag values
	*/

	fmt.Println("GoLog v"+getVersion())

	if config != "" {
	    fmt.Println("["+tools.DateStampAsString()+"] Loading config file....")
	    loadConfig(config)			
	} else {

	  state.MaxBuffLines = buffLines
	  state.CookieDomain = cDomain
	  state.LogBaseDir = logBaseDir	  

	  state.Config["logBaseDir"] = logBaseDir
	  state.Config["ip"] = ip
	  state.Config["port"] = strconv.Itoa(port)
	  state.Config["buffLines"] = strconv.Itoa(buffLines)
	  state.Config["redisDb"] = strconv.Itoa(redisDb)
	  state.Config["flushRedisDb"] = flushRedisDb
	  state.Config["cDomain"] = cDomain
	  state.Config["reportingActive"] = strconv.Itoa(reportingActive)
	  if reportingActive == 1 {
	     state.Config["reportingIp"] = reportingIp
	     state.Config["reportingPort"] = strconv.Itoa(reportingPort)
	  } else {
	    state.Config["reportingActive"] = "0"
	  }
 
	}

	if DEBUG { fmt.Println("Config:", state.Config) }


	// Load the transparent PNG pixel into memory once
	loadOnce.Do(loadPNG)

	// Ensure the GeoIP DB is available
	if tools.FileExists(geoip_db_city) == false {
		if tools.Download(geoip_db_base + geoip_db_city + ".gz") {
			fmt.Println("["+tools.DateStampAsString()+"] Download of " + geoip_db_city + " successful")
		} else {
			fmt.Println("["+tools.DateStampAsString()+"] Could not download " + geoip_db_city)
		}
	}

	geo = loadGeoIpDb(geoip_db_city)

	// Check if the specified directory exists and is writable by the current user
	if _, err := os.Stat(logBaseDir); err != nil {
		if os.IsNotExist(err) {
			err = os.Mkdir(logBaseDir, 0755)
		}
		if err != nil {
			fmt.Println("["+tools.DateStampAsString()+"] Could not created directory: ", logBaseDir)
			fmt.Println("["+tools.DateStampAsString()+"] Please run process as authorized user!\n")
			os.Exit(0)
		}
	}

	fh, _ := os.Create(strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName())
	state.CurrLogFileName = strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName()
	state.CurrLogFileHandle = fh

	defer state.CurrLogFileHandle.Close()


	// Finally, load the redis instance
	/*
	c, redisErr := redis.DialTimeout("tcp", "127.0.0.1:6379", time.Duration(2)*time.Second)
	redisErrHandler(redisErr, "["+tools.DateStampAsString()+"] 1 - tcp connect")
	redisClient = c
	*/

 	// select database and flush it
	/*
        r := redisClient.Cmd("select", redisDb)
        redisErrHandler(r.Err, "[2 - select]")
	if state.Config["flushRedisDb"] == "1" {
           r = redisClient.Cmd("flushdb")
           redisErrHandler(r.Err, "[3 - flushdb]")
	}
	*/

	redisClient = getRedisConnection()

	if state.Config["flushDb"] == "1" {
           redisClient.Append("flushdb")
        }

	// Now set a simple object with a TTL of tomorrow at 00:00:00 so that any stats will get reset
	redisClient.Append("set", "golog_stats_available", 1)
	t := time.Now()
        y,m,d := t.Date()
        expiryTime := time.Date(y, m, d+1, 0, 0, 0, 0, time.Local)
	redisClient.Append("expireat", "golog_stats_available", int(expiryTime.Unix()))	
	redisClient.GetReply()

	// Now close the connection
	redisClient.Close()
	redisClient = nil
	
	//defer redisClient.Close()

	wg := &sync.WaitGroup{}

	// Finally start the reporting server if it's been enabled
	if state.Config["reportingActive"] == "1" {
	   // Start the second process in a seperate go thread so that it can respond and listen only to relevant requests
	   wg.Add(1)
	   go func() {
	   	   //http.HandleFunc("/stats", statsHandler)
           	   err := http.ListenAndServe(state.Config["reportingIp"]+":"+state.Config["reportingPort"], &StatsHandler{})
		   wg.Done()
		   if err != nil {
              	      fmt.Println("GoLog Error:", err)
             	      os.Exit(0)
           	   }
	   }()
           fmt.Println("["+tools.DateStampAsString()+"] Reporting server started on "+state.Config["reportingIp"]+":"+state.Config["reportingPort"])

	}


	wg.Add(1)
	go func() {
	   err := http.ListenAndServe(state.Config["ip"]+":"+state.Config["port"], &LogHandler{})
           if err != nil {
              fmt.Println("GoLog Error:", err)
              os.Exit(0)
           }
	   wg.Done()
	}()

	fmt.Println("["+tools.DateStampAsString()+"] Logging server started on "+state.Config["ip"]+":"+state.Config["port"])


	// Enable conection status PINGing to see if redis connection still alive
	/*
           go func() {
	      for {
	      	status, _ := redisClient.Cmd("PING").Str()
		if status == "PONG" && DEBUG {
	      	   fmt.Println("["+tools.DateStampAsString()+"] Redis Connection Status: OK")
		} else if DEBUG {
		   fmt.Println("["+tools.DateStampAsString()+"] Redis Connection Status: ERROR - "+status)
		}
		time.Sleep(15 * time.Minute)
	      }
	   }()	   
	*/

	wg.Wait()

}
