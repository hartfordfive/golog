golog
=====

A simple self contained web server process that logs data from incoming HTTP request.


Dependencies:
--------------------
Redis Server
Go Redis Client (github.com/fzzy/radix - at least from commit 7059bc0191 or newer)<br/>
Go GeoIP Client (github.com/abh/geoip - at least from commit 6fd87ec2cc or newer)<br/>
MaxMind GeoIP Legacy DBs  (http://geolite.maxmind.com/download/geoip/database/)<br/>


Usage:
--------------------

`go run golog.go -i [IP] -p [PORT] -b [BUFF_LINES] -db [REDIS_DB_INDEX] -d [LOGFILES_DIRECTORY]`

or with compiled binary:

`./golog -i [IP] -p [PORT] -b [BUFF_LINES] -db [REDIS_DB_INDEX] -d [LOGFILES_DIRECTORY]`


Parameter details:
--------------------

`-i` : The IP to start the logging server on (default = 0.0.0.0)<br/>
`-p` : The port on which to listen (default = 80)<br/>
`-b` : The number of lines to store in the buffer before writing to disk (default = 25)<br/>
`-db` : The index number of the redis DB to use (default = 2)<br/>
`-d` : The directory in which the logfiles are to be stored (default = /var/log/golog/)<br/>
`-domain` : The domain for which to set the UDID cookie on<br/>



HTTP URL Format:
--------------------

`http://[DOMAIN]:[PORT]?cid=[CID]&category=[CATEGORY]&action=[ACTION]&label=[LABEL]&value=[VALUE]&rnd=[RAND_INT]`


