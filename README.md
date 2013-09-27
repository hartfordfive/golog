golog
=====

A simple self contained web server process that logs data from incoming HTTP request.


Dependencies:
--------------------
Redis Server<br/>
MaxMind C development library libgeoip-dev<br/>
Go Redis Client (github.com/fzzy/radix - at least from commit 7059bc0191 or newer)<br/>
Go GeoIP Client (github.com/abh/geoip - at least from commit 6fd87ec2cc or newer)<br/>
MaxMind GeoIP Legacy DBs  (http://geolite.maxmind.com/download/geoip/database/)<br/>


Usage:
--------------------

`go run golog.go -i [IP] -p [PORT] -b [BUFF_LINES] -db [REDIS_DB_INDEX] -d [LOGFILES_DIRECTORY] -domain [DOMAIN]` 

or with compiled binary:

`./golog -i [IP] -p [PORT] -b [BUFF_LINES] -db [REDIS_DB_INDEX] -d [LOGFILES_DIRECTORY] -domain [DOMAIN]`


Parameter details:
--------------------

`-version` : Simply prints the current version and exits <br/>
`-i` : The IP to start the logging server on (default = 0.0.0.0)<br/>
`-p` : The port on which to listen (default = 80)<br/>
`-b` : The number of lines to store in the buffer before writing to disk (default = 25)<br/>
`-db` : The index number of the redis DB to use (default = 2)<br/>
`-flushredis` : Setting value to 1 will flush the selected redis DB on startup (default = 0)
`-d` : The directory in which the logfiles are to be stored (default = /var/log/golog/)<br/>
`-domain` : The domain for which to set the UDID cookie on<br/>
`-stats` : Option that specifies if the server will report stats via HTTP (default = 0) <br/>
`-ri` : The IP on which the reporting server will listen <br/>
`-rp` : The port on which the reporting server will listen <br/>


HTTP URL Format:
--------------------

`http://[DOMAIN]:[PORT]?cid=[CID]&category=[CATEGORY]&action=[ACTION]&label=[LABEL]&value=[VALUE]&rnd=[RAND_INT]`


[![githalytics.com alpha](https://cruel-carlota.pagodabox.com/f70384f88bf609745a1ae8a3d9255f01 "githalytics.com")](http://githalytics.com/hartfordfive/golog)
