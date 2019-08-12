# http-proxy


1. Run `docker-compose up -d` for logging and caching containers (or not, still works)
1. Run `meli-proxy.exe`
1. Run `fake_server.exe` in fake_server folder or rerun `meli-proxy.exe -proxy="https://api.mercadolibre.com"`
1. Download bombardier binaries -> https://github.com/codesenberg/bombardier/releases
1. Run `bombardier -c 100 -d 15s -l localhost:12345/sites/MLA/categories`
1. Maybe edit `config.json` and run more requests
1. Check kibana logs on http://localhost:5601


### Improvements
* Status endpoint with stats from elasticsearch 
* Configuration API endpoints
* Local/external cache synchronization 
* Rules: public/authenticated filter, consumer tiers
