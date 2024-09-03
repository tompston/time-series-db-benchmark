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

#### Go benchmarks

The result of manually running `EXPLAIN ANALYZE` on the queries is is in the `read_test.txt` file.

Notes:

- the default value of chunck compression in timescale is changed to one which gives better compression
- mongodb does not use the time series collections because they can't be queried by a single row at a time
- the empty benchmark lines are omitted.
- The mysql version uses a field called `resolution` instead of `interval` because `interval` is a reserved keyword in mysql.

```bash
BenchmarkTimeseries/mysql-insert-500000-rows-10                        1        706494051417 ns/op      540666584 B/op   9506739 allocs/op
BenchmarkTimeseries/mongodb-insert-500000-rows-10                      1        211530664083 ns/op      4064646056 B/op 56505957 allocs/op
BenchmarkTimeseries/pg-ntv-insert-500000-rows-10                       1        181912643125 ns/op      200180400 B/op   6501848 allocs/op
BenchmarkTimeseries/pg-tsc-insert-500000-rows-10                       1        219917379792 ns/op      200208384 B/op   6502129 allocs/op

BenchmarkTimeseries/mysql-upsert-single-4000-rows-10                   1        1383154125 ns/op         4321896 B/op      76004 allocs/op
BenchmarkTimeseries/mongodb-upsert-single-4000-rows-10                 1        1045744583 ns/op        31045960 B/op     416041 allocs/op
BenchmarkTimeseries/pg-ntv-upsert-single-4000-rows-10                  1        1776864291 ns/op         1600000 B/op      52000 allocs/op
BenchmarkTimeseries/pg-tsc-upsert-single-4000-rows-10                  1        2131020208 ns/op         1600000 B/op      52000 allocs/op

BenchmarkTimeseries/mysql-upsert-bulk-4000-rows-10                     2         709444396 ns/op         2529756 B/op      56021 allocs/op
BenchmarkTimeseries/mongodb-upsert-bulk-4000-rows-10                   7         158134631 ns/op        15426505 B/op     176112 allocs/op
BenchmarkTimeseries/pg-ntv-upsert-bulk-4000-rows-10                   19          68316186 ns/op         5768675 B/op      52042 allocs/op
BenchmarkTimeseries/pg-tsc-upsert-bulk-4000-rows-10                    6         199995639 ns/op         5768672 B/op      52042 allocs/op

BenchmarkTimeseries/mysql-get-4000-10                                387           3105753 ns/op         3031493 B/op      40050 allocs/op
BenchmarkTimeseries/mongodb-get-4000-10                              102          12421009 ns/op         5025492 B/op      80200 allocs/op
BenchmarkTimeseries/pg-ntv-get-4000-10                               336           3548450 ns/op         2934926 B/op      16034 allocs/op
BenchmarkTimeseries/pg-tsc-get-4000-10                               248           4571874 ns/op         2934839 B/op      16033 allocs/op

    benchmark_test.go:106: sleeping for 60 sec to get the correct mongodb collection storage size
    benchmark_test.go:109:  * storage size for 500000 rows
    benchmark_test.go:116:      - mysql:    45152 KB
    benchmark_test.go:116:      - mongodb:  41584 KB
    benchmark_test.go:116:      - pg-ntv:   83696 KB
    benchmark_test.go:116:      - pg-tsc:   44544 KB

PASS
ok      timeseries-benchmark    1403.829s
```

- for single upserts, there is not a significant difference between the databases.
- for bulk upserts, the native postgresql is the fastest.
- for read speeds, the timescale extension is the fastest (~ 4x faster than mongodb)
- for table sizes, the timescale version has a slight advantage over mongodb, but a 6x compression over the native postgresql.

#### python read speed

For some reason the python read speed benchmarks have the opposite result. Not sure why. Note that the python script is ran once, so the mean over x runs is not calculated.

```bash
pg_timesale.select_with_limit       116.28 ms
pg_native.select_with_limit           3.81 ms
mongo.select_with_limit               0.11 ms
-----
pg_timesale.find_one                  3.97 ms
pg_native.find_one                    0.63 ms
mongo.find_one                        9.82 ms
```

### Debug commands

```bash
# see size of table in timescale + timing of query
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "SELECT pg_size_pretty(hypertable_size('data_objects')) AS total_size;"
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "EXPLAIN ANALYZE SELECT * FROM data_objects ORDER BY start_time DESC LIMIT 10000;"
# see size of table in postgres + timing of query
docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c "SELECT pg_size_pretty(pg_total_relation_size('data_objects')) AS total_size;"
docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c "EXPLAIN ANALYZE SELECT * FROM data_objects ORDER BY start_time DESC LIMIT 10000;"

# see timing of mongodb query
docker exec timeseries_mongodb mongosh --username test --password test --eval '
    db = db.getSiblingDB("timeseries_benchmark");
    db.data_objects.find({}).sort({ start_time: -1 }).limit(10000).explain("executionStats").executionStats.executionTimeMillis;'
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
