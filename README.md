<h2>GoLog<h2>
=====

A simple self contained web server process that logs data from incoming HTTP request.


##  Notice

This project is no longer maintained.   It has a variety of concurency related bugs that will not be fixed.  For a more recent version of a similar application (with better concucrency), please see the [Logger](https://github.com/hartfordfive/Logger) application.


Dependencies:
--------------------
Redis Server<br/>
MaxMind C development library libgeoip-dev<br/>
Go Redis Client (github.com/fzzy/radix - v0.3.4)<br/>
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
`-conf` : The config file to use to start up the process, instead of specifying all params via command line


HTTP Tracking URL Format:<br/>
----------------------<br/>
Place this url in a tracking pixel somewhere at the bottom of your HTML code<br/>
`http://[DOMAIN]:[PORT]?cid=[CID]&category=[CATEGORY]&action=[ACTION]&label=[LABEL]&value=[VALUE]&rnd=[RAND_INT]`<br/><br/>

`[DOMAIN]` ->  The domain of the website collecting the stats<br/>
`[PORT]` -> The port on which the tracking server is listening<br/>
`[CID]` -> The Client ID of the user account.  (Arbitrary identifier decided by tracking server owner, same concept as a Google Analytics tracking code)<br/>
`[CATEGORY]` -> Based on same concept as the Google event tracking parameters (https://developers.google.com/analytics/devguides/collection/gajs/eventTrackerGuide#SettingUpEventTracking)<br/>
`[ACTION]` -> ** View explanation for `[CATEGORY]` **<br/>
`[LABEL]` -> ** View explanation for `[CATEGORY]` **<br/>
`[VALUE]` -> A numeric value to give this tracking request, typically 1<br/>
`[RAND_INT]` -> A random integer (suggested between 1 and at least 1000000) that prevents this HTTP request from being cached<br/>
<br/>


HTTP Stats Monitoring:<br/>
----------------------<br/>
`http://[BASE_DOMAIN]:[STATS_PORT]/stats` -> Returns a JSON encoded object containing cumulative stats showing the number of visits from each continent and country broken down by hour of the day<br/>

`http://[BASE_DOMAIN]:[STATS_PORT]/statsdevices` -> Returns JSON encoded object containing cumulative stats regarding user agents, such OS, OS version, user agent type, rendering engine, etc.<br/>

`http://[BASE_DOMAIN]:[STATS_PORT]/statsvisitors?domain=[DOMAIN]` -> Returns JSON encoded object containing the pages currently visited for the specified domain<br/>

`http://[BASE_DOMAIN]:[STATS_PORT]/statsgeovisitors?continent_code=[CONTENT_CODE]&country_code=[COUNTRY_CODE]` -> Returns JSON encoded object containing the approximate geo-coordinates of each visitor in the current day<br/>


Stats Monitoring Parameter Details:<br/>
-----------------------------------<br/>

`[DOMAIN]` -> The domain name for which to filter the stats<br/>
`[CONTINENT_CODE]` -> The two letter code of the continent, or the wildcard (*) for all continents<br/>
`[COUNTRY_CODE]` -> The two letter code of the country, or the wildcard (*) for all coutries<br/><br/>

** Please note that specifying * for the CONTINENT_CODE will also directly result in a wildcard for the COUNTRY_CODE<br/><br/>



[![githalytics.com alpha](https://cruel-carlota.pagodabox.com/f70384f88bf609745a1ae8a3d9255f01 "githalytics.com")](http://githalytics.com/hartfordfive/golog)
