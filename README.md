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

Notes:

- the default value of chunck compression in timescale is changed to one which gives better compression
- mongodb does not use the time series collections because they can't be queried by a single row at a time
- the empty benchmark lines are omitted.
- The mysql version uses a field called `resoulution` instead of `interval` because `interval` is a reserved keyword in mysql.

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
- mysql
  - Use `DATETIME` instead of `TIMESTAMP` because `TIMESTAMP` has a range of `1970-2038` and `DATETIME` has a range of `1000-9999` (Error 1292 (22007): Incorrect datetime value: '2038-01-19 04:00:00' for column 'start_time' at row 1).

## Manual EXPLAIN ANALYZE

#### Timescale

```bash
# docker exec timeseries_postgres psql -U test -d timeseries_benchmark -c "EXPLAIN ANALYZE SELECT * FROM data_objects ORDER BY start_time DESC LIMIT 10000;"
----------------------------------------------------------------------------------------------------------------------------------------------------------
 Limit  (cost=0.42..427.37 rows=10000 width=58) (actual time=0.091..2.545 rows=10000 loops=1)
   ->  Index Scan Backward using idx_start_time on data_objects  (cost=0.42..21347.89 rows=500000 width=58) (actual time=0.088..1.944 rows=10000 loops=1)
 Planning Time: 0.550 ms
 Execution Time: 2.888 ms
(4 rows)

# docker exec timeseries_mongodb mongosh --username test --password test --eval '
#   db = db.getSiblingDB("timeseries_benchmark");
#   db.data_objects.find({}).sort({ start_time: -1 }).limit(10000).explain("executionStats").executionStats.executionTimeMillis;'
18



# docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "EXPLAIN ANALYZE SELECT * FROM data_objects ORDER BY start_time DESC LIMIT 10000;"
                     ->  Seq Scan on compress_hyper_68_3040_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2691_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2691_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3039_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2690_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2690_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3038_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2689_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2689_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3037_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2688_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2688_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3036_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2687_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2687_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3035_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2686_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2686_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3034_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2685_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2685_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3033_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2684_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2684_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3032_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2683_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2683_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3031_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2682_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2682_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3030_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2681_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2681_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3029_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2680_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2680_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3028_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2679_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2679_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3027_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2678_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2678_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3026_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2677_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2677_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3025_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2676_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2676_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3024_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2675_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2675_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3023_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2674_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2674_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3022_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2673_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2673_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3021_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2672_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2672_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3020_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2671_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2671_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3019_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2670_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2670_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3018_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2669_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2669_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3017_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2668_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2668_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3016_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2667_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2667_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3015_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2666_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2666_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3014_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2665_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2665_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3013_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2664_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2664_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3012_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2663_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2663_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3011_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2662_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2662_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3010_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2661_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2661_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3009_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2660_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2660_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3008_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2659_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2659_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3007_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2658_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2658_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3006_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2657_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2657_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3005_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2656_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2656_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3004_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2655_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2655_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3003_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2654_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2654_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3002_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2653_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2653_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3001_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2652_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2652_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_3000_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2651_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2651_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2999_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2650_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2650_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2998_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2649_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2649_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2997_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2648_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2648_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2996_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2647_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2647_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2995_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2646_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2646_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2994_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2645_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2645_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2993_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2644_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2644_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2992_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2643_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2643_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2991_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2642_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2642_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2990_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2641_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2641_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2989_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2640_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2640_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2988_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2639_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2639_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2987_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2638_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2638_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2986_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2637_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2637_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2985_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2636_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2636_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2984_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2635_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2635_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2983_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2634_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2634_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2982_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2633_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2633_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2981_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2632_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2632_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2980_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2631_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2631_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2979_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2630_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2630_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2978_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2629_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2629_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2977_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2628_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2628_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2976_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2627_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2627_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2975_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2626_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2626_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2974_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2625_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2625_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2973_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2624_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2624_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2972_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2623_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2623_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2971_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2622_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2622_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2970_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2621_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2621_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2969_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2620_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2620_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2968_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2619_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2619_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2967_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2618_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2618_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2966_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2617_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2617_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2965_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2616_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2616_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2964_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2615_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2615_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2963_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2614_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2614_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2962_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2613_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2613_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2961_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2612_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2612_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2960_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2611_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2611_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2959_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2610_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2610_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2958_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2609_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2609_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2957_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2608_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2608_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2956_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2607_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2607_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2955_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2606_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2606_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2954_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2605_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2605_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2953_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2604_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2604_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2952_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2603_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2603_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2951_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2602_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2602_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2950_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2601_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2601_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2949_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2600_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2600_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2948_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2599_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2599_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2947_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2598_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2598_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2946_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2597_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2597_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2945_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2596_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2596_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2944_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2595_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2595_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2943_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2594_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2594_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2942_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2593_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2593_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2941_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2592_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2592_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2940_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2591_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2591_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2939_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2590_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2590_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2938_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2589_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2589_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2937_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2588_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2588_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2936_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2587_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2587_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2935_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2586_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2586_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2934_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
         ->  Sort  (cost=17872.14..18497.14 rows=250000 width=58) (never executed)
               Sort Key: _hyper_67_2585_chunk.start_time DESC
               ->  Custom Scan (DecompressChunk) on _hyper_67_2585_chunk  (cost=0.05..12.50 rows=250000 width=58) (never executed)
                     ->  Seq Scan on compress_hyper_68_2933_chunk  (cost=0.00..12.50 rows=250 width=220) (never executed)
 Planning Time: 182.599 ms
 Execution Time: 10.471 ms
(1405 rows)

```

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
