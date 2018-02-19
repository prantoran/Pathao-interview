### To run the project
docker-compose up

### The base url and port
- 127.0.0.1:4260

### Exposed Mongo port:
- 4000

### Notes:
- TTL is set to 5 minutes
- Key-Value pair deleted after TTL minutes.
- Deletion occurs when a document retrieved is found to exceed TTL
- Deletion occurs in GET and PATCH calls

### Routes
#### POST
- Pushes new key:value pairs
- sets modified time
- updates new value if key exists

#### PATCH
- sets modified time
- updates new value if key exists

#### GET /values
- returns all values
- updates modified time
#### GET /values?keys=1,2
- retuns values for keys sets
-updates modified time