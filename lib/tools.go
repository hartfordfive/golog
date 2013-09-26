package tools

import (
       "io"
       "net/http"
       "os"
       "bufio"
       "os/exec"
       "net/url"
       "path"
       "fmt"
       "strings"
       "strconv"
       "regexp"
       "time"
)

const (
    ONE_MSEC    = 1000 * 1000
    NUM         = 50
    PIXEL	string = "lib/pixel.png"
)



func FileExists(name string) bool {
   _, err := os.Stat(name)
   res := false
   if err == nil {
       res = true
   } 
   return res
}

func Download(sUrl string) bool{

     u, err := url.Parse(sUrl)
     if err != nil {
        return false
     }

     fileName := path.Base(u.Path)
     out, err := os.Create(fileName)
     defer out.Close()

     resp, err := http.Get(sUrl)
     defer resp.Body.Close()

     _, err = io.Copy(out, resp.Body)
     if err == nil {
        cmd := exec.Command("gunzip", fileName)
        cmd.Run()
        return true
     } else {
       return false
     }

     return false
}

func GetMonthAsIntString(m string) string {

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


func Readln(r *bufio.Reader) (string, error) {
  var (isPrefix bool = true
       err error = nil
       line, ln []byte
      )
  for isPrefix && err == nil {
      line, isPrefix, err = r.ReadLine()
      ln = append(ln, line...)
  }
  return string(ln),err
}

func ParseConfigFile(filePath string) map[string]string{

     f, err := os.Open(filePath)
     if err != nil {
        fmt.Printf("Error! Could not open config file: %v\n", err)
        fmt.Println("")
        os.Exit(0)
     }
     defer f.Close();

     r := bufio.NewReader(f)

     params := map[string]string{
     	    "log_base_dir": "",
     	    "server_ip": "",
	    "server_port": "",
	    "num_buff_lines": "",
	    "redis_db_index": "",
	    "flush_redis_db": "",
	    "cookie_domain": "",
	    "reporting_server_active": "",
	    "reporting_server_ip": "",
	    "reporting_server_port": "",
     }

     for err == nil {
         s,err := Readln(r)
	 if err != nil {
	     break
	 }
         if err == nil && s != "" {
            parts := strings.SplitN(s, "=", 2)
	    _,ok := params[parts[0]]
	    if ok  {
		params[parts[0]] = strings.Trim(parts[1], " ")
	    }
         }
     }

     return params
}


func GetUDID() string{
     f, _ := os.Open("/dev/urandom")
     b := make([]byte, 16)
     f.Read(b)
     f.Close()
     uuid := fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
     return uuid
}


func JoinLists(list1 []string, list2 []string) []string{
     newslice := make([]string, len(list1) + len(list2))
     copy(newslice, list1)
     copy(newslice[len(list1):], list2)
     return newslice
}


func GetUserAgentDetails(ua string) map[string]string{

     //ua = strings.ToLower(ua);

     //matches = regexp.MustCompile(`(?i)(Windows NT\s+[0-9]\.[0-9]|Android|iOS|FirefoxOS|Windows\s*Phone OS [0-9]\.[0-9]|BlackBerry [0-9]{4,4}|BB10)`).FindStringSubmatch(ua)
     matches := regexp.MustCompile(`(?i)(Windows NT|Android|iOS|Firefox|Windows\s*Phone OS|BlackBerry|BB10|iphone os|ipad|ipod|Macintosh)`).FindStringSubmatch(ua)

     deviceData := map[string]string{}

     switch strings.ToLower(matches[1]) {

     	    case "windows nt":
	    	 deviceData["platform"] = "Windows NT"
		 matches = regexp.MustCompile(`(?i)Windows NT\s+([0-9]+\.[0-9]+)`).FindStringSubmatch(ua)		
		 if len(matches) >= 2 {
		    deviceData["os_version"] = matches[1]	    	 
		 }
           
            case "windows phone":
                 deviceData["platform"] = "Windows Phone"
                 matches = regexp.MustCompile(`(?i)Windows Phone OS\s+([0-9]+\.[0-9]+);`).FindStringSubmatch(ua)
		 if len(matches) >= 2 {
                    deviceData["os_version"] = matches[1]
		 }

		 matches = regexp.MustCompile(`(?i)IEMobile\/([0-9]+\.[0-9]+);`).FindStringSubmatch(ua)
		 if len(matches) >= 2 {
		    deviceData["rendering_engine"] = "Trident"
		    deviceData["browser"] = "Internet Explorer Mobile"
		    deviceData["browser_version"] = matches[1]
		 }

	    case "android":
	    	 deviceData["platform"] = "Android"
		 matches = regexp.MustCompile(`(?i)Android\s+([0-9]+\.[0-9]+(\.[0-9]+)*)`).FindStringSubmatch(ua)		 
		 if len(matches) >= 3 {
		    deviceData["os_version"] = matches[1]
		 }

		 matches = regexp.MustCompile(`(?i)(Chrome|Firefox|UCWeb|Maxthon|Opera Mini|Opera|Skyfire)`).FindStringSubmatch(ua)
		 fmt.Println("Browser matches:",matches)
		 if len(matches) >= 1 {
		    deviceData["browser"] = matches[1]
		 }

	    case "ios", "iphone os", "ipad", "ipod":
                 deviceData["platform"] = "iOS"
                 matches = regexp.MustCompile(`(?i)OS\s+([0-9]+_[0-9](_[0-9]+)?)`).FindStringSubmatch(ua)
		 if len(matches) >= 2 {
                    deviceData["os_version"] = strings.Replace(matches[1], "_", ".", -1)
		 }


  	    case "macintosh":
                 deviceData["platform"] = "Mac OSX"
                 matches = regexp.MustCompile(`(?i)Version\/([0-9]+\.[0-9]+\.[0-9]+)\s+Safari\/([0-9]+\.[0-9]+\.[0-9]+)`).FindStringSubmatch(ua)
		 if len(matches) >= 2 {
		    deviceData["browser"] = "Safari"
		    deviceData["rendering_engine"] = "WebKit"
		 }

		 matches = regexp.MustCompile(`(?i)OS\s+X\s+([0-9]+_[0-9]+_[0-9]+)`).FindStringSubmatch(ua)
                 if len(matches) >= 2 {
    		    deviceData["os_version"] = strings.Replace(matches[1], "_", ".", -1)
                 }

            case "firefox":
	    	 // Should match:  mozilla/5.0 (mobile; rv:18.0) gecko/18.0 firefox/18.0
		 matches = regexp.MustCompile(`(?i)mozilla\/5\.0\s+\(([^;]+;)+\s+rv:[0-9]+\.[0-9]+\)\s+gecko\/[0-9]+\.[0-9]+\s+firefox\/([0-9]+\.[0-9]+)`).FindStringSubmatch(ua)
                 //matches = regexp.MustCompile(`(?i)Android\s+([0-9]+\.[0-9]+(\.[0-9]+)*)`).FindStringSubmatch(ua)
                 if len(matches) >= 3 {
		    if matches[1] == "mobile" {
                       deviceData["platform"] = "FirefoxOS"
		       deviceData["os_version"] = matches[2]
		    }
                 }
		 
            case "blackberry", "bb10":
                 deviceData["platform"] = "BlackBerry"
                 matches = regexp.MustCompile(`(?i)(Version/([0-9]+\.[0-9]+(\.[0-9]+)*))`).FindStringSubmatch(ua)
		 if len(matches) >= 2 {
                    deviceData["os_version"] = matches[2]
		 }
		 matches = regexp.MustCompile(`BlackBerry ([0-9]{4,4});`).FindStringSubmatch(ua)
                 if len(matches) >= 2 {
                    deviceData["model"] = matches[1]
		    deviceData["rendering_engine"] = "Mango"
                 }
     }

     _,ok := deviceData["rendering_engine"]
     if strings.Contains(strings.ToLower(ua), "webkit") && !ok {
     	deviceData["rendering_engine"] = "WebKit"
     } else if strings.Contains(strings.ToLower(ua), "gecko") && !ok {
       	deviceData["rendering_engine"] = "Gecko"
     } else if strings.Contains(strings.ToLower(ua), "trident") && !ok {
        deviceData["rendering_engine"] = "Trident"
     } else if strings.Contains(strings.ToLower(ua), "presto") && !ok {
        deviceData["rendering_engine"] = "Presto"
     } else if strings.Contains(strings.ToLower(ua), "netfront") && !ok {
        deviceData["rendering_engine"] = "NetFront"
     } else if strings.Contains(strings.ToLower(ua), "obigo") && !ok {
        deviceData["rendering_engine"] = "Obigo"
     }

     _,ok = deviceData["browser"]
     if deviceData["platform"] == "iOS" && strings.Contains(strings.ToLower(ua), "safari") && !ok {
        deviceData["browser"] = "Safari Mobile"
     } else if strings.Contains(strings.ToLower(ua), "MSIE") && !ok {
       	deviceData["browser"] = "Internet Explorer"
     }


     // Now set the default values if fields are empty
     if _,ok := deviceData["platform"]; !ok {
     	deviceData["platform"] = "Unknown"
     }
     if _,ok := deviceData["os_version"]; !ok {
          deviceData["os_version"] = "Unknown"
     }
     if _,ok := deviceData["model"]; !ok {
          deviceData["model"] = "Unknown"
     }
     if _,ok := deviceData["rendering_engine"]; !ok {
     	  deviceData["rendering_engine"] = "Unknown"
     }
     if _,ok := deviceData["browser"]; !ok {
          deviceData["browser"] = "Unknown"
     }


     return deviceData

}


func YmdToString() string {
     t := time.Now()
     y,m,d := t.Date()
     return strconv.Itoa(y)+fmt.Sprintf("%02d", m)+strconv.Itoa(d)
}

func DateStampAsString() string{
     t := time.Now()
     return YmdToString() + " " + fmt.Sprintf("%02d", t.Hour()) + ":" + fmt.Sprintf("%02d", t.Minute()) + ":" + fmt.Sprintf("%02d", t.Second()) 
}