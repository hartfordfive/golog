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
	"crypto/md5"
)

// Struct that contains the runtime properties including the buffered bytes to be written

const (
	PIXEL         string = "lib/pixel.png"
	PNGPX_B64     string = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGP6zwAAAgcBApocMXEAAAAASUVORK5CYII="
	geoip_db_base string = "http://geolite.maxmind.com/download/geoip/database/"
	geoip_db      string = "GeoIP.dat"
	geoip_db_city string = "GeoLiteCity.dat"
	DEBUG	      bool   = true
	AVG_VISIT_LENGTH     int = 2 // Avg. visit length in minutes
	VERSION_MAJOR  int    = 0
	VERSION_MINOR  int    = 1
	VERSION_PATCH  int    = 4
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

func getInfo(ip string) (string, string, string, string, float32, float32) {

	matches := regexp.MustCompile(`([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)`).FindStringSubmatch(ip)
	if len(matches) >= 1 && geo != nil {
		record := geo.GetRecord(ip)
		if record != nil {
			return record.ContinentCode, record.CountryCode, record.CountryName, record.City, record.Latitude, record.Longitude
		}
	}
	return "", "", "", "", 0.0, 0.0
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

      fh, err := os.OpenFile(filePath, os.O_RDWR|os.O_APPEND, 0640)
      if err != nil {
        //panic(err)
	fh, _ = os.Create(filePath)       
	if DEBUG { fmt.Println("File doesn't exist.  Creating it.") }
      } else {
      	if DEBUG { fmt.Println("Appending to log file.") }
      }
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
		"manufacturer": map[string]int{},
		"ua_type": map[string]int{},
		"bot_type": map[string]int{},
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

func getLiveSiteTraffic(domain string) map[string]int{

     pageVisits := map[string]int{}

     redisClient := getRedisConnection()
     hrsMins := fmt.Sprintf("%02d", time.Now().Hour())
     hrsMins += fmt.Sprintf("%02d", time.Now().Minute())

     // Get all the keys actively visited pages this current minute
     if DEBUG { fmt.Println( "["+tools.DateStampAsString()+"] KEYS page_visitors:"+domain+":"+hrsMins+":*") }
     keys,_ := redisClient.Cmd("KEYS", "page_visitors:"+domain+":"+hrsMins+":*").List()

     for _,v := range keys {
     	 if DEBUG { fmt.Println("[" + tools.DateStampAsString() + "] SCARD "+ v) }
     	 redisClient.Append("SCARD", v)
     }

     for i := 0; i < len(keys); i++ {
     	 r := redisClient.GetReply()
	 val,_ := r.Int()
	 parts := strings.Split(keys[i], ":")
	 pageVisits[parts[3]] = val
     }

     if DEBUG { fmt.Println("[" + tools.DateStampAsString() + "] Redis Set:", pageVisits) }

     redisClient.Close()
     return pageVisits     

}


func getLiveIpStats(countryCode string) map[string]map[string]int {
     
     ipStats := map[string]map[string]int{}

     redisClient := getRedisConnection()

     if len(countryCode) == 0 || countryCode == "" {
     	countryCode = "*"
     }

     if DEBUG { fmt.Println( "["+tools.DateStampAsString()+"] KEYS ip_hits:*") }

     keyPattern := ""
     if countryCode == "*" {
     	keyPattern = "ip_hits:*"
     } else {
       keyPattern = "ip_hits:"+countryCode+":*"
     }

     keys,_ := redisClient.Cmd("KEYS", keyPattern).List()


     for _,v := range keys {
         redisClient.Append("ZRANGE", v, 0, -1, "WITHSCORES")
     }

     for i := 0; i < len(keys); i++ {

            r := redisClient.GetReply()
            val,_ := r.List()
	    parts := strings.Split(keys[i], ":")
	    numElems := len(val)
	    for j := 0; j < numElems; j++ {
	    	
            	_,ok := ipStats[parts[1]]
            	if !ok {
               	   ipStats[parts[1]] = make(map[string]int)
            	}
		intVal,_ := strconv.Atoi(val[j+1])
            	ipStats[parts[1]][val[j]] = intVal	
		j++

	   }

     }

     return ipStats

}


func getLiveSiteGeoTraffic(continentCode string, countryCode string) map[string]map[string][]string{

     geoVisits := map[string]map[string][]string{}


     redisClient := getRedisConnection()

     hrsMins := fmt.Sprintf("%02d", time.Now().Hour())
     hrsMins += fmt.Sprintf("%02d", time.Now().Minute())

     // Get all the keys actively visited pages this current minute
     if continentCode != "*" && len(continentCode) != 2 {
       continentCode = "NA"
     }

     if countryCode != "*" && len(countryCode) != 2 {
       countryCode = "CA"
     }

 
     
     if (continentCode == "*" && countryCode == "*") || continentCode == "*" {

     	if DEBUG { fmt.Println( "["+tools.DateStampAsString()+"] KEYS geo_visitors:*") }
     	keys,_ := redisClient.Cmd("KEYS", "geo_visitors:*").List()

	for _,v := range keys {
            redisClient.Append("SMEMBERS", v)
     	}

	for i := 0; i < len(keys); i++ {
            r := redisClient.GetReply()
            val,_ := r.List()
            parts := strings.Split(keys[i], ":")	    
	    _,ok := geoVisits[parts[1]]
	    if !ok {
	       geoVisits[parts[1]] = make(map[string][]string)
	    }
            geoVisits[parts[1]][parts[2]] = val	    
     	}


     } else if continentCode != "*" && countryCode == "*" {

       	// Fetch the data for only a given continent ([CONTINENT]:*)     
        if DEBUG { fmt.Println( "["+tools.DateStampAsString()+"] KEYS geo_visitors:"+continentCode+":*") }
        keys,_ := redisClient.Cmd("KEYS", "geo_visitors:"+continentCode+":*").List()

        for _,v := range keys {
            redisClient.Append("SMEMBERS", v)
        }

        for i := 0; i < len(keys); i++ {
            r := redisClient.GetReply()
            val,_ := r.List()
            parts := strings.Split(keys[i], ":")
            _,ok := geoVisits[continentCode]
            if !ok {
               geoVisits[continentCode] = make(map[string][]string)
            }
            geoVisits[continentCode][parts[2]] = val
        }
       


     } else if countryCode != "*" {

       // Fetch the coords of users found inside of specific country (*:[COUNTRY])       	
        if DEBUG { fmt.Println( "["+tools.DateStampAsString()+"] KEYS geo_visitors:*:"+countryCode) }
        keys,_ := redisClient.Cmd("KEYS", "geo_visitors:*:"+countryCode).List()

        for _,v := range keys {
            redisClient.Append("SMEMBERS", v)
        }

	for i := 0; i < len(keys); i++ {
            r := redisClient.GetReply()
            val,_ := r.List()
            parts := strings.Split(keys[i], ":")
            _,ok := geoVisits[parts[1]]
            if !ok {
               geoVisits[parts[1]] = make(map[string][]string)
            }
            geoVisits[parts[1]][parts[2]] = val
        }


     } else {

     }

     fmt.Println("\tDATA:",geoVisits)

     redisClient.Close()
     return geoVisits

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

	continent := ""
	countryCode := ""
	//country := ""
	city := ""
	ip := ""
	ua := ""
	udid := ""
	cid := ""
	category := ""
	action := ""
	label := ""
	value := ""
	lat := float32(0.0)
	lon := float32(0.0)


	_, ok := params["ip"]
        if ok {
          ip = strings.Replace(params.Get("ip"), "~", "", -1)
        } else if req.Header.Get("X-Forwarded-For") != "" {
	  ip = req.Header.Get("X-Forwarded-For")
	} else {
	  ip = req.RemoteAddr
	}

	_, ok = params["ua"]
        if ok {
           ua = strings.Replace(params.Get("ua"), "~", "-", -1)
        } else if ua == "" {
           ua = req.Header.Get("User-Agent")
        }

	/*** If the UA still isn't set, attempt to detect from other headers ***/
	if ua == "" {
	   ua = req.Header.Get("X-OperaMini-Phone-UA")	   
	}
	if ua == "" {
           ua = req.Header.Get("X-Original-User-Agent")
        }
	if ua == "" {
           ua = req.Header.Get("X-Device-User-Agent")
        }

	// Extract the IP and get its related info
        matches := regexp.MustCompile(`([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)`).FindStringSubmatch(ip)
	
	if len(matches) >= 1 {

		continent, countryCode, _, city, lat, lon = getInfo(matches[1])
		ip = matches[1]
		currHour := strconv.Itoa(time.Now().Hour())

		t := time.Now()
		y,m,d := t.Date()
		expiryTime := time.Date(y, m, d+1, 0, 0, 0, 0, time.Local)


		//dateStr, _ := redisClient.Cmd("GET", "next_dumpfile_date").Str()
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


			dateStr, _ := redisClient.Cmd("get", "next_dumpfile_date").Str()
        		 _ = writeToFile(state.Config["logBaseDir"] + "_daily_geo_stats-" + dateStr +".json", string(dataToDump))

			 // Now dump the device stats		
			  if DEBUG {
                           fmt.Println("["+tools.DateStampAsString()+"] NOTICE: Writing device stats from previous day to JSON file in "+state.Config["logBaseDir"])
                        }

                         dataToDump,_ = json.Marshal(getDeviceStats())
			 _ = writeToFile(state.Config["logBaseDir"] + "_daily_device_stats-" + dateStr + ".json", string(dataToDump))
			 

			 dataToDump,_ = json.Marshal(getLiveSiteGeoTraffic("*","*"))
			 _ = writeToFile(state.Config["logBaseDir"] + "_daily_geocoord_stats-" + dateStr + ".json", string(dataToDump))


			 dataToDump,_ = json.Marshal(getLiveIpStats("*"))
                         _ = writeToFile(state.Config["logBaseDir"] + "_daily_ip_stats-" + dateStr + ".json", string(dataToDump))



			 // Now delete all keys in hashed set and			
			 // set the "golog_stats_available" key again with the proper expiry time
			 
			 for i := 0; i < len(resKeys); i++ {
			     redisClient.Append("ZREMRANGEBYRANK", resKeys[i], 0, -1)
			 }
			 
			 // Flush the remainder of the keys		 
			 redisClient.Append("FLUSHDB")
			 redisClient.Append("SET", "golog_stats_available", 1)
			 redisClient.Append("SET", "next_dumpfile_date", tools.YmdToString())
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
					   
		      	if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis Operation [ZINCRBY "+keyPrefix+":"+currHour+" 1 "+member+"]") }
		      	redisClient.Append("ZINCRBY", keyPrefix+":"+currHour , 1, member)		      	
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

		   if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis Operation [ZINCRBY device_stats:manufacturer:"+tools.YmdToString() + " 1 " + deviceDetails["manufacturer"]+"]")  }
                   redisClient.Append("ZINCRBY", "device_stats:manufacturer:"+tools.YmdToString(), 1, deviceDetails["manufacturer"])

		   if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis Operation [ZINCRBY device_stats:ua_type:"+tools.YmdToString() + " 1 " + deviceDetails["ua_type"]+"]")  }
                   redisClient.Append("ZINCRBY", "device_stats:ua_type:"+tools.YmdToString(), 1, deviceDetails["ua_type"])

		   if _,ok := deviceDetails["bot_type"]; ok{
		      if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis Operation [ZINCRBY device_stats:bot_type:"+tools.YmdToString() + " 1 " + deviceDetails["bot_type"]+"]")  }
                      redisClient.Append("ZINCRBY", "device_stats:bot_type:"+tools.YmdToString(), 1, deviceDetails["bot_type"])
		   }

		   if deviceDetails["ua_type"] == "Mobile" {
		      if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Redis Operation [ZINCRBY device_stats:model:"+tools.YmdToString() + " 1 " + deviceDetails["model"]+"]")  }
                      redisClient.Append("ZINCRBY", "device_stats:model:"+tools.YmdToString(), 1, deviceDetails["model"])
		   }


		   //****************** Increment the related IP hits *****************************
		   redisClient.Append("ZINCRBY", "ip_hits:"+countryCode+":"+tools.YmdToString(), 1, ip)


		   // ***************** Now populate the live visitor stats ************************
		    hrs := fmt.Sprintf("%02d", time.Now().Hour())
		    mins := time.Now().Minute()
		    h := md5.New()
		    io.WriteString(h, req.RemoteAddr + "~" + req.Header.Get("User-Agent"))		    
		    md5Hash := fmt.Sprintf("%x", h.Sum(nil))

		    // This block adds the page increment for the next avg. visit lenght in minutes
		     ref := req.Header.Get("Referer")
		     urlHost := ""
		     urlPath := ""
		     if ref != "" {
		     	up,_ := url.Parse(ref)
			urlPath = up.Path
			urlHost = up.Host
		     } else {
		       urlHost = "default"
		       urlPath = "/"
		     }

		    for i := 0; i < AVG_VISIT_LENGTH; i++ {		    
			 mins := mins+i
			 minutes := fmt.Sprintf("%02d", mins)
			 
		    	if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] SADD page_visitors:" + urlHost + ":" + hrs+minutes + ":" + urlPath + " " + md5Hash)  }
			if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] EXPIRE page_visitors:"+ urlHost + ":"+hrs+minutes+":"+ urlPath + " " + strconv.Itoa((i+1)*60))  }
		    	redisClient.Append("SADD", "page_visitors:"+ urlHost +":" + hrs + minutes + ":" + urlPath, md5Hash)
		   	redisClient.Append("EXPIRE", "page_visitors:"+ urlHost +":" + hrs + minutes + ":" + urlPath, ((i+1)*60))
		   }

		   // And now store the visitor GeoLocation stats in the necessary set
		   if DEBUG { fmt.Println("["+tools.DateStampAsString()+ "] SADD geo_visitors:" + continent + ":" + countryCode + " " + fmt.Sprint(lat) +","+ fmt.Sprint(lon)) }
		   redisClient.Append("SADD", "geo_visitors:"+ continent + ":" + countryCode , fmt.Sprint(lat) +","+ fmt.Sprint(lon) )
		   if DEBUG { fmt.Println("["+tools.DateStampAsString()+ "] EXPIRE geo_visitors:"+ continent + ":" + countryCode, expiryTime.Unix()) }
                   redisClient.Append("EXPIRE", "geo_visitors:"+ continent + ":" + countryCode, expiryTime.Unix())


		   // Finally flush the buffer of operations to the redis server
		   redisClient.GetReply()		   		   		   

		   // Now close the redis connection
		   redisClient.Close()

		} // END of "redisClient != nil" condition

	}

	_, ok = params["udid"]
        if ok {
                udid = strings.Replace(params.Get("udid"), "~", "-", -1)
        }

	ln += "[" + strconv.Itoa(ts) + "] ~ " + ip + " ~ " + countryCode + " ~ " + city + " ~ "
	
	 _, ok = params["cid"]
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
        if cookie != "" && udid == "" {
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


	ln += cid + " ~ " + udid + " ~ " + category + " ~ " + action + " ~ " + label + " ~ " + value + " ~ " + ua + "\n"

	state.BuffLines = append(state.BuffLines, ln)
	state.BuffLineCount++

	// If there are 25 lines to be written, flush the buffer to the logfile
	
	if bl,_ := strconv.Atoi(state.Config["buffLines"]); state.BuffLineCount >= bl {

	   	if DEBUG { 
		   fmt.Println("\tWritting buffer to disk. ("+ strconv.Itoa(bl) +" lines total)" ) 		
		}		

		if strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName() != state.CurrLogFileName {
		
		        if DEBUG { fmt.Println("\t Updated Filename:", strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName()) }

			fh, err := os.OpenFile(strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName(), os.O_RDWR|os.O_APPEND, 0660)
			if err != nil {			        
			       if DEBUG { fmt.Println("\tCould not open file to append data, attempting to create file..") }
			       fh,_ = os.Create(strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName())	      	   
				
	   	        }

			state.CurrLogFileName = strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName()
			//state.CurrLogFileHandle = fh
			defer fh.Close()
		}

		totalBytes := 0
		fh, err := os.OpenFile(state.CurrLogFileName, os.O_RDWR|os.O_APPEND, 0660)
		if err == nil {
		   for i := 0; i < state.BuffLineCount; i++ {
		   	nb, err3 := fh.WriteString(state.BuffLines[i])
			if err3 != nil && DEBUG {
			   fmt.Println("\t Could not write to file "+state.CurrLogFileName+":", err3)
			}  
			totalBytes += nb
		    }
		    fh.Sync()
		    defer fh.Close();
		    if DEBUG { fmt.Println("\t Wrote to:", state.CurrLogFileName) }
		    // Empty the buffer and reset the buff line count to 0
		    state.BuffLineCount = 0
		    state.BuffLines = []string{}
		}
	}

	// Finally, return the tracking pixel and exit the request.
	res.Header().Set("Cache-control", "public, max-age=0")
	res.Header().Set("Content-Type", "image/png")	
	res.Header().Set("Server","GoLog/"+getVersion())
	output,_ := base64.StdEncoding.DecodeString(PNGPX_B64)
	io.WriteString(res, string(output))

	return

}


func (sh *StatsHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
         
      if req.URL.Path != "/stats" && req.URL.Path != "/statsdevices" && 
      	 req.URL.Path != "/statsvisitors" && req.URL.Path != "/statsgeovisitors" &&
	 req.URL.Path != "/statsip" {
      	  res.WriteHeader(http.StatusNotFound)
	  res.Header().Set("Cache-control", "public, max-age=0")
     	  res.Header().Set("Content-Type", "text/html")          
	  res.Header().Set("Server","GoLog/"+getVersion())
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
	    res.Header().Set("Server","GoLog/"+getVersion())
     	    if err1 == nil {
               fmt.Fprintf(res, string(data1))
     	    } else {
       	      fmt.Fprintf(res, "{\"status\": \"error\"}")
     	    }

	    redisClient.Close()

	} else if req.URL.Path == "/statsdevices" {

	       //redisClient := getRedisConnection()

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
	     res.Header().Set("Server","GoLog/"+getVersion())

     	    if err == nil {
               fmt.Fprintf(res, string(data))
	    } else {
               fmt.Fprintf(res, "{\"status\": \"error\"}")
       	    }

	    //redisClient.Close()

	} else if req.URL.Path == "/statsvisitors" {

	       qs := req.URL.Query()
               fmt.Println(qs)
	     
	    data,err := json.Marshal(getLiveSiteTraffic(qs.Get("domain")))
             res.Header().Set("Cache-control", "public, max-age=0")
             res.Header().Set("Content-Type", "application/json")
	     res.Header().Set("Server","GoLog/"+getVersion())
            if err == nil {
               fmt.Fprintf(res, string(data))
            } else {
               fmt.Fprintf(res, "{\"status\": \"error\"}")
            }

            //redisClient.Close()


	} else if req.URL.Path == "/statsgeovisitors" {

               qs := req.URL.Query()

            data,err := json.Marshal(getLiveSiteGeoTraffic(qs.Get("continent_code"), qs.Get("country_code")))

             res.Header().Set("Cache-control", "public, max-age=0")
             res.Header().Set("Content-Type", "application/json")
	     res.Header().Set("Server","GoLog/"+getVersion())

            if err == nil {
               fmt.Fprintf(res, string(data))
            } else {
               fmt.Fprintf(res, "{\"status\": \"error\"}")
            }

            //redisClient.Close()


        } else if req.URL.Path == "/statsip" {

	  qs := req.URL.Query()
            data,err := json.Marshal(getLiveIpStats(qs.Get("country_code")))

             res.Header().Set("Cache-control", "public, max-age=0")
             res.Header().Set("Content-Type", "application/json")
             res.Header().Set("Server","GoLog/"+getVersion())

            if err == nil {
               fmt.Fprintf(res, string(data))
            } else {
               fmt.Fprintf(res, "{\"status\": \"error\"}")
            }


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

	fh, err := os.OpenFile(strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName(), os.O_RDWR|os.O_APPEND, 0660)
        if err != nil {
           fh, _ = os.Create(strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName())
        }

	state.CurrLogFileName = strings.TrimRight(state.Config["logBaseDir"], "/") + "/" + getLogfileName()
	//state.CurrLogFileHandle = fh

	defer fh.Close()


	// Finally, load the redis instance
	redisClient = getRedisConnection()

	if state.Config["flushDb"] == "1" {
	   if DEBUG { fmt.Println("["+tools.DateStampAsString()+"] Flushing Redis DB...") }	   
           redisClient.Append("flushdb")
        }

	// Now set a simple object with a TTL of tomorrow at 00:00:00 so that any stats will get reset
	redisClient.Append("set", "golog_stats_available", 1)
	t := time.Now()
        y,m,d := t.Date()
        expiryTime := time.Date(y, m, d+1, 0, 0, 0, 0, time.Local)
	redisClient.Append("expireat", "golog_stats_available", int(expiryTime.Unix()))	
	redisClient.Append("set", "next_dumpfile_date", tools.YmdToString())
	redisClient.GetReply()

	// Now close the connection
	redisClient.Close()
	redisClient = nil
	
	
	wg := &sync.WaitGroup{}

	// Finally start the reporting server if it's been enabled
	if state.Config["reportingActive"] == "1" {
	   // Start the second process in a seperate go thread so that it can respond and listen only to relevant requests
	   wg.Add(1)
	   go func() {
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

	wg.Wait()

}
