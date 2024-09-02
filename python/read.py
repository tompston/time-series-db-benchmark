from datetime import datetime, timedelta, timezone
import random, time
from dataclasses import dataclass
from typing import List

# postgresql
from psycopg2 import sql
import psycopg2

# mongodb
from pymongo import MongoClient, UpdateOne
from pymongo.collection import Collection

DB_NAME = "timeseries_benchmark"
DB_USERNAME = "test"
DB_PASSWORD = "test"
DB_TABLE_NAME = "data_objects"

PORT_MONGO = 5551
PORT_POSTGRES = 5552
PORT_TIMESCALE = 5553

BASE_TIME = datetime(2022, 1, 1, 0, 0, 0, tzinfo=timezone.utc)

@dataclass
class DataObject:
    """ Object used to simulate time-series data """
    created_at: datetime
    updated_at: datetime
    start_time: datetime # start of the measurement
    interval: int # in milliseconds
    area: str  # area of the measurement
    source: str # source of the data
    value: float # value of the measurement
    

class PostgresDb:
    def __init__(self, conn_str: str, using_timescaledb: bool):
        self.conn = psycopg2.connect(conn_str)
        self.cursor = self.conn.cursor()
        self.using_timescaledb = using_timescaledb

    def close(self):
        self.cursor.close()
        self.conn.close()
        
        
    def select_with_limit(self, limit: int):
        self.cursor.execute(f"SELECT * FROM {DB_TABLE_NAME} ORDER BY start_time DESC LIMIT %s", (limit,))
        rows = self.cursor.fetchall()
        return rows
    
    def find_one(self):
        self.cursor.execute(f"SELECT * FROM {DB_TABLE_NAME} WHERE area = 'lv' AND source = 'source-of-data' AND start_time = %s", (BASE_TIME,))
        return self.cursor.fetchone()
    

class MongoDB:
    def __init__(self, uri: str):
        self.client = MongoClient(uri)

    def close(self):
        self.client.close()
        
    def select_with_limit(self, limit: int):
        coll = self.client[DB_NAME][DB_TABLE_NAME]
        rows = coll.find().sort("start_time", -1).limit(limit)
        return rows
    
    def find_one(self):
        coll = self.client[DB_NAME][DB_TABLE_NAME]
        return coll.find_one({
            "area": "lv",
            "source": "source-of-data",
            "start_time": BASE_TIME
        })
            

def benchmark_function(runner, func, *args, **kwargs):
    """Helper function to benchmark a specific function."""
    runner.bench_func(name=f"{func.__name__}", func=lambda: func(*args, **kwargs))
    
    # # Measure memory usage
    # mem_usage = memory_usage((func, args, kwargs), max_iterations=1)
    # print(f"{func.__name__} memory usage: {max(mem_usage) - min(mem_usage)} MiB")

    # # Measure memory allocations
    # tracemalloc.start()
    # func(*args, **kwargs)
    # current, peak = tracemalloc.get_traced_memory()
    # tracemalloc.stop()
    # print(f"{func.__name__} current memory usage: {current / 10**6} MB; Peak: {peak / 10**6} MB")


def measure_time(method_name, method, *args, **kwargs):
    """Measure the execution time of a method in milliseconds and print it."""
    start_time = time.perf_counter()
    method(*args, **kwargs)
    end_time = time.perf_counter()
    elapsed_time = (end_time - start_time) * 1000  # convert to milliseconds

    # Print the time with formatted alignment
    print(f"{method_name:<30}  {elapsed_time:>10.2f} ms")


if __name__ == "__main__":
    mongo = MongoDB(f"mongodb://{DB_USERNAME}:{DB_PASSWORD}@localhost:{PORT_MONGO}/")
    pg_native = PostgresDb(f"dbname={DB_NAME} user={DB_USERNAME} password={DB_PASSWORD} host=localhost port={PORT_POSTGRES}", using_timescaledb=False)
    pg_timesale = PostgresDb(f"dbname={DB_NAME} user={DB_USERNAME} password={DB_PASSWORD} host=localhost port={PORT_TIMESCALE}", using_timescaledb=True)
    
    NUM_ROWS = 1_000

    measure_time("pg_timesale.select_with_limit", pg_timesale.select_with_limit, NUM_ROWS)
    measure_time("pg_native.select_with_limit", pg_native.select_with_limit, NUM_ROWS)
    measure_time("mongo.select_with_limit", mongo.select_with_limit, NUM_ROWS)
    print("-----")
    measure_time("pg_timesale.find_one", pg_timesale.find_one)
    measure_time("pg_native.find_one", pg_native.find_one)
    measure_time("mongo.find_one", mongo.find_one)
        
    mongo.close()
    pg_native.close()
    pg_timesale.close()