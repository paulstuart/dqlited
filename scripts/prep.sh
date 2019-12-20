#!/bin/bash

set -ex

cd $(dirname $0)/..

export PATH=.:$PATH

# our sample table for testing
dqlited adhoc "drop table if exists model"
dqlited adhoc "create table model (id integer primary key, name text, value text)"
dqlited adhoc "insert into model (name, value) values('Bowie', 'Rock God')"

# our migrated postgres dump
dqlited load -f sqlite.sql


