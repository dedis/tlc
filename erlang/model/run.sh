#!/bin/sh
erl -make && erl -noshell -run qsc tests -run init stop
