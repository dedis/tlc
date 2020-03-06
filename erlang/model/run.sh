#!/bin/sh
erl -make && erl -noshell -run qsc test -run init stop
