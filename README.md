## Time series database benchmarking

The following project tries to benchmark the following methods on 3 database instances:

- mysql
- mongodb
- postgresql
- postgresql with timescale extension

Each database instance is ran through docker and uses the non-default ports to avoid conflicts with local database instances.

**NOTE.** Create an issue if you see a mistake or have a suggestion.

The following methods are benchmarked:

- insert 250k rows (aka upserts on empty table)
- upsert single row at a time
- upsert a bulk of rows
- read x rows with a limit of y and sort descending by start_time.

The data format for all of the tables is the same (excluding the id field between mongodb and postgres implementations)

```json
{
  "created_at": "2021-09-01T00:00:00Z", // TIMESTAMPTZ | mongodb ISODate
  "updated_at": "2021-09-01T00:00:00Z", // TIMESTAMPTZ | mongodb ISODate
  "start_time": "2021-09-01T00:00:00Z", // TIMESTAMPTZ | mongodb ISODate
  "interval": TIME_IN_MILLISECONDS,     // BIGINT
  "area": "area",                       // TEXT
  "source": "source",                   // TEXT
  "value": 0.0,                         // double precision
}
```

Upserts are done using an `start_time`, `interval` and `area` filter.

The same dat chunks get inserted into all of the databases.

## Commands

```bash
# start docker in the background
sudo docker compose up -d
# run the go benchmarks
cd go
go test -benchmem -run=^$ -bench ^BenchmarkTimeseries$ timeseries-benchmark -v -count=1 -timeout=0
# run the python script to test read speed
cd python
python3 read.py

# reset docker (uninstall every image and container)
sudo docker stop $(sudo docker ps -aq)
sudo docker rm $(sudo docker ps -aq)
sudo docker rmi $(sudo docker images -q)
sudo docker rmi -f $(sudo docker images -q)
sudo docker volume rm $(docker volume ls -q)
```

## Results

> The result of manually running `EXPLAIN ANALYZE` on the queries is is in the `read_test.txt` file.

```bash
 go $ go test -benchmem -run=^$ -bench ^BenchmarkTimeseries$ timeseries-benchmark -v -count=1 -timeout=0
goos: darwin
goarch: arm64
pkg: timeseries-benchmark
BenchmarkTimeseries
BenchmarkTimeseries/mysql-insert-250000-rows-10                        1        393717472208 ns/op      270376864 B/op    4753748 allocs/op
BenchmarkTimeseries/mongodb-insert-250000-rows-10                      1        107140407042 ns/op      2032751896 B/op  28254084 allocs/op
BenchmarkTimeseries/pg-ntv-insert-250000-rows-10                       1        80756041500 ns/op       100079632 B/op   3250795 allocs/op
BenchmarkTimeseries/pg-tsc-insert-250000-rows-10                       1        96840892958 ns/op       100099872 B/op   3250988 allocs/op

BenchmarkTimeseries/mysql-upsert-single-4000-rows-10                   1        1588451000 ns/op         4321896 B/op      76004 allocs/op
BenchmarkTimeseries/mongodb-upsert-single-4000-rows-10                 1        1129163375 ns/op        31062512 B/op     416154 allocs/op
BenchmarkTimeseries/pg-ntv-upsert-single-4000-rows-10                  1        1183699083 ns/op         1600000 B/op      52000 allocs/op
BenchmarkTimeseries/pg-tsc-upsert-single-4000-rows-10                  1        1498572959 ns/op         1600000 B/op      52000 allocs/op

BenchmarkTimeseries/mysql-upsert-bulk-4000-rows-10                     2         721893917 ns/op         2529884 B/op      56023 allocs/op
BenchmarkTimeseries/mongodb-upsert-bulk-4000-rows-10                   7         152042810 ns/op        15323329 B/op     176117 allocs/op
BenchmarkTimeseries/pg-ntv-upsert-bulk-4000-rows-10                   19          55011397 ns/op         5768672 B/op      52042 allocs/op
BenchmarkTimeseries/pg-tsc-upsert-bulk-4000-rows-10                    6         186201875 ns/op         5768672 B/op      52042 allocs/op

    benchmark_test.go:90:  * storage size for pg-tsc, 250000 rows, before compression: 47640

BenchmarkTimeseries/mysql-get-4000-10                                380           3121183 ns/op         3031533 B/op      40051 allocs/op
BenchmarkTimeseries/mongodb-get-4000-10                               99          11677680 ns/op         5057506 B/op      80200 allocs/op
BenchmarkTimeseries/pg-ntv-get-4000-10                               511           2216196 ns/op         2934812 B/op      16032 allocs/op
BenchmarkTimeseries/pg-tsc-get-4000-10                               378           4087209 ns/op         2934892 B/op      16033 allocs/op

    benchmark_test.go:112: sleeping for 60 sec to get the correct mongodb collection storage size
    benchmark_test.go:115:  * storage size for 250000 rows
    benchmark_test.go:122:      - mysql:    23632 KB
    benchmark_test.go:122:      - mongodb:  21728 KB
    benchmark_test.go:122:      - pg-ntv:   39776 KB
    benchmark_test.go:122:      - pg-tsc:   5328 KB
PASS
ok      timeseries-benchmark    761.087s

```

- in terms of inserts, mysql was the slowest
- for single upserts, there is not a significant difference between the databases.
- for bulk upserts, the native postgresql is the fastest.
- for read speeds, the native postgresql version is the fastest.
- for table sizes, the timescale version can have a smaller size than the native postgresql version, but the efficiency of the compression is vastly dependent on the chunk size.

Notes:

- the default value of chunck compression in timescale is changed to one which gives better compression
- mongodb does not use the time series collections because they can't be queried by a single row at a time
- the empty benchmark lines are omitted.
- The mysql version uses a field called `resolution` instead of `interval` because `interval` is a reserved keyword in mysql.

### EXPLAIN ANALYZE queries

To get specific info about how long the queries took on the database level, i ran the `read_test.sh` script post benchmarking. The results between native postgres and timescale are not great. Native postgres `SELECT *` queries otperform timescale by at least 2x. All of the logs of the explain queries can be inspected in the file.

```bash
* postgres select with limit
  Planning Time: 0.585 ms
  Execution Time: 2.399 ms

* timescaledb compressed -> select with limit
  Planning Time: 13.739 ms
  Execution Time: 6.818 ms

* timescale decompressed -> select with limit
  Planning Time: 3.955 ms
  Execution Time: 3.160 ms
```

### Debug commands

```bash
# see size of table in timescale + timing of query
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "SELECT pg_size_pretty(hypertable_size('data_objects')) AS total_size;"
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "EXPLAIN ANALYZE SELECT * FROM data_objects ORDER BY start_time DESC LIMIT 10000;"
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "\d data_objects"

# see size of table in postgres + timing of query
docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c "SELECT pg_size_pretty(pg_total_relation_size('data_objects')) AS total_size;"
docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c "EXPLAIN ANALYZE SELECT * FROM data_objects ORDER BY start_time DESC LIMIT 10000;"
docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c "\d data_objects"

# see timing of mongodb query
docker exec timeseries_mongodb mongosh --username test --password test --eval '
    db = db.getSiblingDB("timeseries_benchmark");
    db.data_objects.find({}).sort({ start_time: -1 }).limit(10000).explain("executionStats").executionStats.executionTimeMillis;'


docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c 'SELECT *
  FROM timescaledb_information.dimensions
  WHERE hypertable_name = 'metrics';'

```

### Gotchas

- mongodb
  - The statistics about the mongodb collection seem to be incorrect just after inserting the data. The `totalSize` value updates after some time, once the records are inserted. This is why there is a pause before reading the collection size.
  - **The displayed storage size may not be correct.** While running the benchmarks, i found that in some cases the displayed storage of the mongodb collection did not increase when the number of records increased by 10x. So i don't think the displayed storage size can be trusted fully.
- timescale
  - the size of the chunk matters. From my understanding the default is 7 days. In this benchmark we save 1 hour resolution data, for which `30 days` otperforms compression of `7 days` with a big margin.
    - 7 days -> 4864 kb
    - 30 days -> 1576 kb
    - 60 days -> 1064 kb
  - To get the table size of the timescaledb, the default `SELECT pg_size_pretty(pg_total_relation_size($1)) AS total_size;` query does not return the correct. I found this out when the table size returned from this query did not change once i benchmarked the size on varying number of rows (1k -> 10k).
  - The compression of timescale does not get applied immediately after the inserts. That's why we need to trigger it manually.
  - possible cause for concern (not sure if this is fixed) [compress_chunk() blocks other queries on the table for a long time](https://github.com/timescale/timescaledb/issues/2732)
  - adjustments based on interval blog post[link](https://mail-dpant.medium.com/my-experience-with-timescaledb-compression-68405425827)
- mysql
  - Use `DATETIME` instead of `TIMESTAMP` because `TIMESTAMP` has a range of `1970-2038` and `DATETIME` has a range of `1000-9999` (Error 1292 (22007): Incorrect datetime value: '2038-01-19 04:00:00' for column 'start_time' at row 1).

<!--
source ~/python-envs/sant/bin/activate
/Users/tompston/python-envs/sant/bin

# see chunk info and compression status
SELECT chunk_schema, chunk_name, compression_status,
        pg_size_pretty(before_compression_total_bytes) AS size_total_before,
        pg_size_pretty(after_compression_total_bytes) AS size_total_after
    FROM chunk_compression_stats('public.data_objects')
    ORDER BY chunk_name;

# get the total compression
SELECT
    pg_size_pretty(before_compression_total_bytes) as before,
    pg_size_pretty(after_compression_total_bytes) as after
 FROM hypertable_compression_stats('public.data_objects');

 -->
