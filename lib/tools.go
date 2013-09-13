package tools

import (
       "io"
       //"fmt"
       "net/http"
       "os"
       "os/exec"
       "net/url"
       "path"
       //"time"
)

const (
    ONE_MSEC    = 1000 * 1000
    _TIOCGWINSZ = 0x5413    // On OSX use 1074295912. Thanks zeebo
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

/*
func LastModified(name string) int {

     lastMod := 0
     info, err := os.Stat(name)
     if err != nil {
       lastMod = 0
     } else {
       lastMod = (time.Nanoseconds()-info.Atime_ns)/1000000000
     }
     return lastMod
}
*/

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


