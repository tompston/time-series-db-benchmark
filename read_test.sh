#!/bin/bash

# delete the previous read_test.txt if it exists
rm -f ./read_test.txt

echo " ~ mongodb" >> read_test.txt
docker exec timeseries_mongodb mongosh --username test --password test --eval '
    db = db.getSiblingDB("timeseries_benchmark");
    db.data_objects.find({}).sort({ start_time: -1 }).limit(10000).explain("executionStats").executionStats.executionTimeMillis;' >> read_test.txt

# add a new line
echo "" >> read_test.txt


# ------------------------------------------------
echo "  * postgres select with limit" >> read_test.txt
docker exec timeseries_postgres psql -U test -d timeseries_benchmark \
    -c "EXPLAIN ANALYZE SELECT * FROM data_objects ORDER BY start_time DESC LIMIT 10000;"  >> read_test.txt

echo "  * postgres sum value with limit" >> read_test.txt
docker exec timeseries_postgres psql -U test -d timeseries_benchmark \
    -c "EXPLAIN ANALYZE SELECT SUM(value) AS total_value FROM data_objects LIMIT 10000;"  >> read_test.txt


# ------------------------------------------------

echo " ~ timescaledb version" >> read_test.txt
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c \
    "SELECT default_version, installed_version FROM pg_available_extensions where name = 'timescaledb';" >> read_test.txt


echo "  * timescaledb compressed -> select with limit" >> read_test.txt
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "SELECT compress_chunk(c) from show_chunks('data_objects') c;"
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark \
    -c "EXPLAIN ANALYZE SELECT * FROM data_objects ORDER BY start_time DESC LIMIT 10000;"  >> read_test.txt

echo "  * timescale compressed -> sum value with limit" >> read_test.txt
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark \
    -c "EXPLAIN ANALYZE SELECT SUM(value) AS total_value FROM data_objects LIMIT 10000;"  >> read_test.txt

# ------------------------------------------------
echo "  * timescale decompressed -> select with limit" >> read_test.txt
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark -c "SELECT decompress_chunk(c) from show_chunks('data_objects') c;"
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark \
    -c "EXPLAIN ANALYZE SELECT * FROM data_objects ORDER BY start_time DESC LIMIT 10000;"  >> read_test.txt

echo "  * timescale decompressed -> sum value with limit" >> read_test.txt
docker exec timeseries_timescaledb psql -U test -d timeseries_benchmark \
    -c "EXPLAIN ANALYZE SELECT SUM(value) AS total_value FROM data_objects LIMIT 10000;"  >> read_test.txt