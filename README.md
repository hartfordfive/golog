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

`golog -i [IP] -p [PORT] -b [BUFF_LINES] -db [REDIS_DB_INDEX] -d [LOGFILES_DIRECTORY]`


HTTP URL Format:
--------------------

`http://[DOMAIN]:[PORT]?cid=[CID]&category=[CATEGORY]&action=[ACTION]&label=[LABEL]&value=[VALUE]&rnd=[RAND_INT]`


