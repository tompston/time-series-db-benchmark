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

Upserts are don using an `start_time`, `interval` and `area` filter.

## Commands

```bash
# start docker in the background
sudo docker compose up -d
# run the go benchmarks
cd go
go test -benchmem -run=^$ -bench ^BenchmarkTimeseries$ timeseries-benchmark -v -count=1
# run the python script to test read speed
cd python
python3 main.py

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

- timescale results use 30 day compression for chunks
- mongodb does not use the time series collections because they can't be queried by a single row at a time
- the empty benchmark lines are omitted.

```bash
goos: darwin
goarch: arm64
pkg: timeseries-benchmark
BenchmarkTimeseries
BenchmarkTimeseries/mongodb-upsert-single-10                   1        3660702667 ns/op        81749872 B/op    1131090 allocs/op
BenchmarkTimeseries/pg-ntv-upsert-single-10                    1        3435178500 ns/op         6409688 B/op     140044 allocs/op
BenchmarkTimeseries/pg-tsc-upsert-single-10                    1        3750847458 ns/op         6420336 B/op     140142 allocs/op

BenchmarkTimeseries/mongodb-upsert-bulk-10                     3         413395472 ns/op        49944133 B/op     440150 allocs/op
BenchmarkTimeseries/pg-ntv-upsert-bulk-10                      6         208642778 ns/op        17143922 B/op     140056 allocs/op
BenchmarkTimeseries/pg-tsc-upsert-bulk-10                      2         520394042 ns/op        17145708 B/op     140060 allocs/op

BenchmarkTimeseries/mongodb-get-1000-10                      225           4753699 ns/op         1154879 B/op      20176 allocs/op
BenchmarkTimeseries/pg-ntv-get-1000-10                       772           1543699 ns/op          658344 B/op       4026 allocs/op
BenchmarkTimeseries/pg-tsc-get-1000-10                      1122           1234099 ns/op          658343 B/op       4026 allocs/op

    benchmark_test.go:88: Sleeping for 30 sec to get the correct mongodb collection storage size
    benchmark_test.go:91:  * storage size for 10000 rows
    benchmark_test.go:98:       - mongodb: 1752 KB
    benchmark_test.go:98:       - pg-ntv: 9640 KB
    benchmark_test.go:98:       - pg-tsc: 1576 KB
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

### Gotchas

- mongodb
  - The statistics about the mongodb collection seem to be incorrect just after inserting the data. The `totalSize` value updates after some time, once the records are inserted. This is why there is a 30sec sleep in the script.
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




SELECT
    column_name,
    pg_size_pretty(avg(pg_column_size(column_name::text))) AS avg_size,
    pg_size_pretty(max(pg_column_size(column_name::text))) AS max_size,
    pg_size_pretty(min(pg_column_size(column_name::text))) AS min_size
FROM
    (SELECT
        your_column1 AS column_name
     FROM data_objects) subquery
GROUP BY column_name;


SELECT
    timeseries_benchmark,
    pg_size_pretty(table_size) AS table_size,
    pg_size_pretty(indexes_size) AS indexes_size,
    pg_size_pretty(total_size) AS total_size
FROM (
    SELECT
        timeseries_benchmark,
        pg_table_size(timeseries_benchmark) AS table_size,
        pg_indexes_size(timeseries_benchmark) AS indexes_size,
        pg_total_relation_size(timeseries_benchmark) AS total_size
    FROM (
        SELECT ('"' || table_schema || '"."' || timeseries_benchmark || '"') AS timeseries_benchmark
        FROM information_schema.tables
    ) AS all_tables
    ORDER BY total_size DESC
) AS pretty_sizes;
 -->
