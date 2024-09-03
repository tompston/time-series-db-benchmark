## Time series database benchmarking

The following project tries to benchmark the following methods on 3 database instances:

- mongodb
- postgresql
- postgresql with timescale extension

Each database instance is ran through docker and uses the non-default ports to avoid conflicts with local database instances.

**NOTE.** Create an issue if you see a mistake or have a suggestion.

The following methods are benchmarked:

- upsert single row at a time
- upsert a bulk of rows
- read x rows with a limit of 1000 and sort descending by start_time.

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

## Commands

```bash
# start docker in the background
sudo docker compose up -d
# run the go benchmarks
cd go
go test -benchmem -run=^$ -bench ^BenchmarkTimeseries$ timeseries-benchmark -v -count=1
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

#### Go benchmarks

Notes:

- the default value of chunck compression in timescale is changed to one which gives better compression
- mongodb does not use the time series collections because they can't be queried by a single row at a time
- the empty benchmark lines are omitted.
- The mysql version uses a field called `resoulution` instead of `interval` because `interval` is a reserved keyword in mysql.

```bash
goos: darwin
goarch: arm64
pkg: timeseries-benchmark
BenchmarkTimeseries
BenchmarkTimeseries/mongodb-upsert-single-10                   1        3580940583 ns/op        81749416 B/op    1131013 allocs/op
BenchmarkTimeseries/pg-ntv-upsert-single-10                    1        3293623834 ns/op         4003696 B/op     130030 allocs/op
BenchmarkTimeseries/pg-tsc-upsert-single-10                    1        4055551458 ns/op         4013808 B/op     130124 allocs/op
BenchmarkTimeseries/mysql-upsert-single-10                     1        15338941584 ns/op       10812808 B/op     190112 allocs/op

BenchmarkTimeseries/mongodb-upsert-bulk-10                     3         388987458 ns/op        49942456 B/op     440148 allocs/op
BenchmarkTimeseries/pg-ntv-upsert-bulk-10                      8         175830583 ns/op        14742259 B/op     130061 allocs/op
BenchmarkTimeseries/pg-tsc-upsert-bulk-10                      3         500025292 ns/op        14740981 B/op     130049 allocs/op
BenchmarkTimeseries/mysql-upsert-bulk-10                       1        1894484167 ns/op         6322584 B/op     140023 allocs/op

BenchmarkTimeseries/mongodb-get-1000-10                      336           3478583 ns/op         1170932 B/op      20176 allocs/op
BenchmarkTimeseries/pg-ntv-get-1000-10                      1404            812259 ns/op          658326 B/op       4026 allocs/op
BenchmarkTimeseries/pg-tsc-get-1000-10                      1192           1009948 ns/op          658327 B/op       4026 allocs/op
BenchmarkTimeseries/mysql-get-1000-10                        890           1361952 ns/op          682950 B/op      10044 allocs/op
    benchmark_test.go:95: Sleeping for 60 sec to get the correct mongodb collection storage size
    benchmark_test.go:98:  * storage size for 10000 rows
    benchmark_test.go:105:      - mongodb: 1664 KB
    benchmark_test.go:105:      - pg-ntv: 11264 KB
    benchmark_test.go:105:      - pg-tsc: 1448 KB
    benchmark_test.go:105:      - mysql: 1792 KB
```

- for single upserts, there is not a significant difference between the databases.
- for bulk upserts, the native postgresql is the fastest.
- for read speeds, the timescale extension is the fastest (~ 4x faster than mongodb)
- for table sizes, the timescale version has a slight advantage over mongodb, but a 6x compression over the native postgresql.

#### python read speed

For some reason the python read speed benchmarks have the opposite result. Not sure why. Note that the python script is ran once, so the mean over x runs is not calculated.

```bash
mongo.select_with_limit               0.10 ms
pg_native.select_with_limit           4.60 ms
pg_timesale.select_with_limit        10.31 ms
```

### Debug commands

```bash
# see size of table in timescale
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "SELECT pg_size_pretty(hypertable_size('data_objects')) AS total_size;"
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "EXPLAIN ANALYZE SELECT * FROM data_objects;"
# see size of table in postgres
docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c "SELECT pg_size_pretty(pg_total_relation_size('data_objects')) AS total_size;"
docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c "EXPLAIN ANALYZE SELECT * FROM data_objects;"
```

### Gotchas

- mongodb
  - The statistics about the mongodb collection seem to be incorrect just after inserting the data. The `totalSize` value updates after some time, once the records are inserted. This is why there is a 30sec sleep in the script.
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

<!--
source ~/python-envs/sant/bin/activate
/Users/tompston/python-envs/sant/bin


psql -U test -d timeseries_benchmark -W
SELECT hypertable_size('data_objects');
SELECT * FROM hypertable_detailed_size('data_objects') ORDER BY node_name;
SELECT * FROM hypertable_approximate_detailed_size('data_objects');


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



use timeseries_benchmark
db.data_objects.find({}).explain("executionStats").executionStats
db.data_objects.find({}).explain("executionStats").executionStats.executionTimeMillis


psql -U test -d timeseries_benchmark -W
EXPLAIN ANALYZE SELECT * FROM data_objects;

go test -benchmem -run=^$ -bench ^BenchmarkTimeseries$ timeseries-benchmark -v -count=1 -timeout=0


go install github.com/gotesttools/gotestfmt/v2/cmd/gotestfmt@latest
go test -benchmem -run=^$ -bench ^BenchmarkTimeseries$ timeseries-benchmark -count=1 -timeout=0 | gotestfmt


SELECT hypertable_size(data_objects) AS total_size;
docker exec -it timeseries_timescaledb psql -U test -d timeseries_benchmark -c "SELECT * FROM hypertable_detailed_size('data_objects') ORDER BY node_name;"


 -->
