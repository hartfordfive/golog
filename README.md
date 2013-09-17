golog
=====

A simple self contained web server process that logs data from incoming HTTP request.


Dependancies:
--------------------
Redis



Usage:
--------------------

`go run golog.go -i [IP] -p [PORT] -b [BUFF_LINES] -db [REDIS_DB_INDEX] -d [LOGFILES_DIRECTORY]`

or with compiled binary:

`./golog -i [IP] -p [PORT] -b [BUFF_LINES] -db [REDIS_DB_INDEX] -d [LOGFILES_DIRECTORY]`


Parameter details:
--------------------

`-i` : The IP to start the logging server on (default = 0.0.0.0)
`-p` : The port on which to listen (default = 80)
`-b` : The number of lines to store in the buffer before writing to disk (default = 25)
`-db` : The index number of the redis DB to use (default = 2)
`-d` : The directory in which the logfiles are to be stored (default = /var/log/golog/)



HTTP URL Format:
--------------------

`http://[DOMAIN]:[PORT]?cid=[CID]&category=[CATEGORY]&action=[ACTION]&label=[LABEL]&value=[VALUE]&rnd=[RAND_INT]`


