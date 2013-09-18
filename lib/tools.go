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
)

const (
    ONE_MSEC    = 1000 * 1000
    NUM         = 50
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

     params := map[string]string{}
     for err == nil {
         s,err := Readln(r)
	 if err != nil {
	     break
	 }
         if err == nil && s != "" {
            parts := strings.SplitN(s, "=", 2)
            if len(parts) == 2 {
	      //fmt.Println("Key:", parts[0], "Val:", parts[1])
              params[strings.Trim(parts[0], " ")] = params[strings.Trim(parts[1], " ")]
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
