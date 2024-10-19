## Time series database benchmarking

The following project runs benchmarks for the following databases:

- mysql
- mongodb
- postgresql
- postgresql with timescale extension (version 2.16.1)

Each database instance is ran through docker and uses the non-default ports to avoid conflicts with local database instances.

**NOTE.** Create an issue if you see a mistake or have a suggestion.

Benchmarked methods:

- insert x rows (aka upserts on empty table, with indexes)
- upsert single row at a time
- upsert a bulk of rows
- read x rows with a limit of y and sort descending by start_time.

The data format for all of the tables is the same (excluding the id field between mongodb and postgres implementations, and the name of the interval field in mysql).

```json
{
  "created_at": "2021-09-01T00:00:00Z", // TIMESTAMPTZ | mongodb ISODate
  "updated_at": "2021-09-01T00:00:00Z", // TIMESTAMPTZ | mongodb ISODate
  "start_time": "2021-09-01T00:00:00Z", // TIMESTAMPTZ | mongodb ISODate
  "interval": TIME_IN_MILLISECONDS,     // BIGINT (renamed to resolution in mysql)
  "area": "area",                       // TEXT
  "source": "source",                   // TEXT
  "value": 0.0,                         // double precision
}
```

Upserts are done using an `start_time`, `interval` and `area` filter.

The same data chunks get inserted into all of the databases.

As the benchmark results between timescale and postgres did not match the results of the official timescale [blogpost](https://www.timescale.com/blog/postgresql-timescaledb-1000x-faster-queries-90-data-compression-and-much-more/), I asked for help / validation on [reddit](https://www.reddit.com/r/PostgreSQL/comments/1ftnlu3/native_postgresql_version_faster_than_timescaledb/).

## Commands

```bash
# start docker in the background
sudo docker compose up -d
# run the go benchmarks
cd go
go test -benchmem -run=^$ -bench ^BenchmarkTimeseries$ timeseries-benchmark -v -count=1 -timeout=0

# reset docker (uninstall every image and container)
sudo docker stop $(sudo docker ps -aq)
sudo docker rm $(sudo docker ps -aq)
sudo docker rmi $(sudo docker images -q)
sudo docker rmi -f $(sudo docker images -q)
sudo docker volume rm $(docker volume ls -q)
```

## Results

```bash
 go $ go test -benchmem -run=^$ -bench ^BenchmarkTimeseries$ timeseries-benchmark -v -count=1 -timeout=0
goos: darwin
goarch: arm64
pkg: timeseries-benchmark
BenchmarkTimeseries
BenchmarkTimeseries/mysql-insert-100000-rows-10                        1        101650284500 ns/op      108107240 B/op   1900985 allocs/op
BenchmarkTimeseries/mongodb-insert-100000-rows-10                      1        41199354291 ns/op       813491216 B/op  11302331 allocs/op
BenchmarkTimeseries/pg-ntv-insert-100000-rows-10                       1        32611100375 ns/op       40034880 B/op    1300323 allocs/op
BenchmarkTimeseries/pg-tsc-insert-100000-rows-10                       1        52285019042 ns/op       40052832 B/op    1300511 allocs/op

BenchmarkTimeseries/mysql-upsert-single-4000-rows-10                   1        1865689000 ns/op         4321896 B/op      76004 allocs/op
BenchmarkTimeseries/mongodb-upsert-single-4000-rows-10                 1        1112991375 ns/op        31069864 B/op     416178 allocs/op
BenchmarkTimeseries/pg-ntv-upsert-single-4000-rows-10                  1        1378900750 ns/op         1600000 B/op      52000 allocs/op
BenchmarkTimeseries/pg-tsc-upsert-single-4000-rows-10                  1        2096648666 ns/op         1600000 B/op      52000 allocs/op

BenchmarkTimeseries/mysql-upsert-bulk-4000-rows-10                     2         757385021 ns/op         2529884 B/op      56023 allocs/op
BenchmarkTimeseries/mongodb-upsert-bulk-4000-rows-10                   6         167808972 ns/op        14574984 B/op     176101 allocs/op
BenchmarkTimeseries/pg-ntv-upsert-bulk-4000-rows-10                   19          59175759 ns/op         5768672 B/op      52042 allocs/op
BenchmarkTimeseries/pg-tsc-upsert-bulk-4000-rows-10                    2         515012583 ns/op         5773888 B/op      52090 allocs/op

    benchmark_test.go:90:  * storage size for pg-tsc, 100000 rows, before compression: 23568

BenchmarkTimeseries/mysql-get-4000-10                                376           3211083 ns/op         3031471 B/op      40051 allocs/op
BenchmarkTimeseries/mongodb-get-4000-10                               85          12326166 ns/op         5018889 B/op      80200 allocs/op
BenchmarkTimeseries/pg-ntv-get-4000-10                               510           2367323 ns/op         2934839 B/op      16033 allocs/op
BenchmarkTimeseries/pg-tsc-get-4000-10                               309           3820294 ns/op         2934888 B/op      16033 allocs/op

    benchmark_test.go:112: sleeping for 60 sec to get the correct mongodb collection storage size
    benchmark_test.go:115:  * storage size for 100000 rows
    benchmark_test.go:122:      - mysql: 10272 KB
    benchmark_test.go:122:      - mongodb: 7704 KB
    benchmark_test.go:122:      - pg-ntv: 20208 KB
    benchmark_test.go:122:      - pg-tsc: 8392 KB
PASS
ok      timeseries-benchmark    306.979s

```

- in terms of inserts (upserts on initial run), mysql was the slowest and native postgres was the fastest. Timescale took ~1.6x longer than the native postgres.
- for single upserts, mongodb was the fastest and timescale was the slowest.
- for bulk upserts, the native postgresql is the fastest, while the timescale version was ~8 times slower than the native postgresql version.
- for read speeds, the native postgresql version ~1.5x faster than the timescale version while running from go.
  - for some reason the EXPLAIN ANALYZE queries show that the native postgresql reads take roughly 2.5ms, while 
    - compressed version takes ~70ms (28x slower)
    - decompressed version takes ~20ms (8x slower)
- for table sizes, the timescale version can have a smaller size than the native postgresql version, but the efficiency of the compression is vastly dependent on the chunk size. 

Notes:

- the default value of chunck compression in timescale is changed to one which gives better compression
- mongodb does not use the time series collections because they can't be queried by a single row at a time
- the empty benchmark lines are omitted.
- The mysql version uses a field called `resolution` instead of `interval` because `interval` is a reserved keyword.

### EXPLAIN ANALYZE queries

To get specific info about how long the queries took on the database level, i ran the `read_test.sh` script post benchmarking. The results between native postgres and timescale are not great. Native postgres `SELECT *` queries otperform timescale by at least 2x. All of the logs of the explain queries can be inspected in the file.

```bash
* postgres select with limit
 Planning Time: 0.323 ms
 Execution Time: 1.932 ms
(4 rows)


 ~ timescaledb version
 default_version | installed_version 
-----------------+-------------------
 2.16.1          | 2.16.1
(1 row)

* timescaledb compressed -> select with limit
 Planning Time: 63.068 ms
 Execution Time: 7.791 ms
(293 rows)

* timescale decompressed -> select with limit
 Planning Time: 16.949 ms
 Execution Time: 3.828 ms
(75 rows)
```

### Debug commands

```bash
# find the version of timescaledb
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "SELECT default_version, installed_version FROM pg_available_extensions where name = 'timescaledb';"

# see size of table in timescale + timing of query
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "SELECT pg_size_pretty(hypertable_size('data_objects')) AS total_size;"
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "\d data_objects"

# see size of table in postgres + timing of query
docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c "SELECT pg_size_pretty(pg_total_relation_size('data_objects')) AS total_size;"
docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c "\d data_objects"

docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c 'SELECT *
  FROM timescaledb_information.dimensions
  WHERE hypertable_name = 'metrics';'
```

SELECT default_version, installed_version FROM pg_available_extensions where name = 'timescaledb';

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


docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c 'SELECT
    pg_size_pretty(before_compression_total_bytes) as before,
    pg_size_pretty(after_compression_total_bytes) as after
 FROM hypertable_compression_stats('public.data_objects');'

 -->
