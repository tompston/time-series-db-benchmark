## Time-series upsert performance & compression

A small check to benchmark the performance of upsert queries for time-series data in:

- mongodb
- postgresql
- postgresql with timescaledb extension

**NOTE** that this was written in a hurry and the end results should be taken with a grain of salt.

### Setup

- mongodb on localhost:27017, no auth
- postgresql on localhost:5432, user: postgres, password: postgres, db: test
- install timescaledb extension on postgresql

### Result

```
go test -benchmem -run=^$ -bench ^BenchmarkDbs$ ts-benchmark

BenchmarkDbs/MongoDB-compound-single-upsert-10         	       1	1394460875 ns/op	76327976 B/op	 1140562 allocs/op
BenchmarkDbs/Postgres-native-single-10                 	       1	1376155583 ns/op	 2488368 B/op	   90038 allocs/op
BenchmarkDbs/Postgres-timescale-single_uspert-10       	       1	1849769834 ns/op	 2480400 B/op	   89993 allocs/op
BenchmarkDbs/Postgres-timescale-batch_upsert-10        	       3	 412889889 ns/op	11025413 B/op	   90033 allocs/op

Table size summary for 10000 records:
------------------------
MongoDB: 			1308 kb
Postgres (native): 		2168 kb
Postgres (timescale): 		24 kb
```
