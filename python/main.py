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
    
def generate_fake_data(num_objects: int) -> List[DataObject]:
    """ generate an array of DataObject instances """
    
    rows = []
    
    for i in range(num_objects):
        current_time = datetime.now(timezone.utc)  # Current UTC time for 'created_at' and 'updated_at'
        
        obj = DataObject(
            created_at=current_time,
            updated_at=current_time,
            start_time=BASE_TIME + timedelta(hours=i),  # Incrementing start_time by 1 hour for each object
            source="source-of-data",
            interval=3600000,  # 1 hour in milliseconds
            area="lv",
            value=random.uniform(-7000, 7000)
        )
        rows.append(obj)
    
    return rows


class PostgresDb:
    def __init__(self, conn_str: str, using_timescaledb: bool):
        self.conn = psycopg2.connect(conn_str)
        self.cursor = self.conn.cursor()
        self.using_timescaledb = using_timescaledb

    def close(self):
        self.cursor.close()
        self.conn.close()
        
    def table_setup(self):
        
        self.cursor.execute(f"DROP TABLE IF EXISTS {DB_TABLE_NAME}")
        self.conn.commit()
        
        self.cursor.execute(
            sql.SQL(
                f"""
                CREATE TABLE IF NOT EXISTS {DB_TABLE_NAME} (
                    created_at  TIMESTAMPTZ         NOT NULL,
                    updated_at  TIMESTAMPTZ         NOT NULL,
                    start_time  TIMESTAMPTZ         NOT NULL,
                    interval    BIGINT              NOT NULL,
                    area        TEXT                NOT NULL,
                    source      TEXT                NOT NULL,
                    value       DOUBLE PRECISION    NOT NULL,
                    PRIMARY KEY (start_time, interval, area)
                )
                """
            )
        )
        self.conn.commit()
        
        
        if self.using_timescaledb:
            # create the hypertable on the start_time field
            self.cursor.execute(sql.SQL(f"SELECT create_hypertable('{DB_TABLE_NAME}', 'start_time');"))
            
            
            self.conn.commit()

        # create a unique index on the start_time + interval + area fields
        self.cursor.execute(sql.SQL(f"CREATE UNIQUE INDEX IF NOT EXISTS idx_data_objects ON {DB_TABLE_NAME} (start_time, interval, area)"))
        self.conn.commit()
        
        # create an index on the start_time field
        self.cursor.execute(sql.SQL(f"CREATE INDEX IF NOT EXISTS idx_start_time ON {DB_TABLE_NAME} (start_time)"))
        self.conn.commit()
    
        
    def table_size_in_kb(self):
        if self.using_timescaledb:
            self.cursor.execute(f"SELECT hypertable_size('{DB_TABLE_NAME}') AS total_size;")
            return self.cursor.fetchone()[0] / 1024
        
        
        self.cursor.execute(f"SELECT pg_total_relation_size('{DB_TABLE_NAME}') / 1024 AS size_kb;")
        size_kb = self.cursor.fetchone()[0]
        return size_kb
    
    def get_num_rows(self):
        self.cursor.execute(f"SELECT COUNT(*) FROM {DB_TABLE_NAME}")
        num_rows = self.cursor.fetchone()[0]
        return num_rows

    def upsert_single(self, rows: List[DataObject]):
        for obj in rows:
            self.cursor.execute(
                sql.SQL(
                    f"""
                    INSERT INTO {DB_TABLE_NAME} (created_at, updated_at, start_time, interval, area, source, value)
                    VALUES (%s, %s, %s, %s, %s, %s, %s)
                    ON CONFLICT (start_time, interval, area)
                    DO UPDATE SET
                        updated_at = EXCLUDED.updated_at,
                        source = EXCLUDED.source,
                        value = EXCLUDED.value
                    """
                ),
                (obj.created_at, obj.updated_at, obj.start_time, obj.interval, obj.area, obj.source, obj.value)
            )

        self.conn.commit()
        
    def upsert_batch(self, rows: List[DataObject]):
        # Construct the SQL query with placeholders for each row
        insert_query = sql.SQL(
            f"""
            INSERT INTO {DB_TABLE_NAME} (created_at, updated_at, start_time, interval, area, source, value)
            VALUES %s
            ON CONFLICT (start_time, interval, area)
            DO UPDATE SET
                updated_at = EXCLUDED.updated_at,
                source = EXCLUDED.source,
                value = EXCLUDED.value
            """
        )
        
        # Convert the DataObject list to a list of tuples
        values = [(obj.created_at, obj.updated_at, obj.start_time, obj.interval, obj.area, obj.source, obj.value) for obj in rows]
        
        # Use psycopg2's `execute_values` to insert multiple rows at once
        from psycopg2.extras import execute_values
        execute_values(self.cursor, insert_query, values)
        self.conn.commit()
        
    def select_with_limit(self, limit: int):
        self.cursor.execute(f"SELECT * FROM {DB_TABLE_NAME} ORDER BY start_time DESC LIMIT %s", (limit,))
        rows = self.cursor.fetchall()
        return rows
    

class MongoDB:
    def __init__(self, uri: str):
        self.client = MongoClient(uri)

    def close(self):
        self.client.close()
        
    def table_setup(self):
        # drop the previous table
        self.client[DB_NAME].drop_collection(DB_TABLE_NAME)
        
        coll = self.client[DB_NAME][DB_TABLE_NAME]
        # create a compound index on the start_time + interval + area fields
        coll.create_index([("start_time", 1), ("interval", 1), ("area", 1)], unique=True)
        # create an index on the start_time field
        coll.create_index([("start_time", 1)])
        
    def table_size_in_kb(self):
        coll_stats = self.client[DB_NAME].command("collStats", DB_TABLE_NAME)
        size_kb = coll_stats.get("totalSize", 0) / 1024  # size is in bytes, so convert to KB
        return size_kb
    
    def get_num_rows(self):
        coll = self.client[DB_NAME][DB_TABLE_NAME]
        num_rows = coll.count_documents({})
        return num_rows
    
    def upsert_single(self, rows: List[DataObject]):
        coll = self.client[DB_NAME][DB_TABLE_NAME]
        for obj in rows:
            coll.update_one(
                {"start_time": obj.start_time, "interval": obj.interval, "area": obj.area},
                {"$set": obj.__dict__},
                upsert=True
            )

    def upsert_batch(self, rows: List[DataObject]):
        ops = []
        coll = self.client[DB_NAME][DB_TABLE_NAME]
        
        for obj in rows:
            filter = {"start_time": obj.start_time, "interval": obj.interval, "area": obj.area}
            update = {"$set": obj.__dict__}
            ops.append(UpdateOne(filter, update, upsert=True))

        # execute the batch of operations
        if ops:
            coll.bulk_write(ops)
            
    def select_with_limit(self, limit: int):
        coll = self.client[DB_NAME][DB_TABLE_NAME]
        rows = coll.find().sort("start_time", -1).limit(limit)
        # print(len(list(rows)))
        return rows
            

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
    # Generate data objects
    NUM_OBJECTS = 10000
    fake_data = generate_fake_data(NUM_OBJECTS)
    
    # MongoDB
    mongo = MongoDB(f"mongodb://{DB_USERNAME}:{DB_PASSWORD}@localhost:{PORT_MONGO}/")
    # # mongo.table_setup()
    # mongo.upsert_single(fake_data)
    # mongo.upsert_batch(fake_data)
    # mongo.select_with_limit(1000)
    # print(f"mongodb table size: {mongo.table_size_in_kb()} KB. Number of rows: {mongo.get_num_rows()}")
    
    pg_native = PostgresDb(f"dbname={DB_NAME} user={DB_USERNAME} password={DB_PASSWORD} host=localhost port={PORT_POSTGRES}", using_timescaledb=False)
    # # pg_native.table_setup()
    # pg_native.upsert_single(fake_data)
    # pg_native.upsert_batch(fake_data)
    # pg_native.select_with_limit(1000)
    # print(f"postgres table size: {pg_native.table_size_in_kb()} KB. Number of rows: {pg_native.get_num_rows()}")
    
    
    pg_timesale = PostgresDb(f"dbname={DB_NAME} user={DB_USERNAME} password={DB_PASSWORD} host=localhost port={PORT_TIMESCALE}", using_timescaledb=True)
    # # pg_timesale.table_setup()
    # pg_timesale.upsert_single(fake_data)
    # pg_timesale.upsert_batch(fake_data)
    # pg_timesale.select_with_limit(1000)
    # print(f"timescale table size: {pg_timesale.table_size_in_kb()} KB. Number of rows: {pg_timesale.get_num_rows()}")
    
    NUM_ROWS = 1_000

    # measure_time("mongo.upsert_single", mongo.upsert_single, fake_data)
    # measure_time("mongo.upsert_batch", mongo.upsert_batch, fake_data)
    measure_time("mongo.select_with_limit", mongo.select_with_limit, NUM_ROWS)
    print("------")
    
    # measure_time("pg_native.upsert_single", pg_native.upsert_single, fake_data)
    # measure_time("pg_native.upsert_batch", pg_native.upsert_batch, fake_data)
    measure_time("pg_native.select_with_limit", pg_native.select_with_limit, NUM_ROWS)
    print("------")
    
    # measure_time("pg_timesale.upsert_single", pg_timesale.upsert_single, fake_data)
    # measure_time("pg_timesale.upsert_batch", pg_timesale.upsert_batch, fake_data)
    measure_time("pg_timesale.select_with_limit", pg_timesale.select_with_limit, NUM_ROWS)
    print("------")
    
    print("mongo table size: ", mongo.table_size_in_kb())
    print("pg_native table size: ", pg_native.table_size_in_kb())
    print("pg_timesale table size: ", pg_timesale.table_size_in_kb())
    
    
    mongo.close()
    pg_native.close()
    pg_timesale.close()