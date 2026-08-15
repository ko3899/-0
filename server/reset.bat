@echo off
REM reset database schema for reseeding (drop all tables then recreate)
set PGPASSWORD=hotel123
D:\pgsql\bin\psql.exe -h 127.0.0.1 -p 5432 -U hotel -d hotel_management -c "DROP SCHEMA public CASCADE;"
D:\pgsql\bin\psql.exe -h 127.0.0.1 -p 5432 -U hotel -d hotel_management -c "CREATE SCHEMA public;"
echo RESET_DONE
