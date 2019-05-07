#!/bin/bash
#
# Code coverage generation
#
# parameters: <required pkg name> <required coverage output file> <optional html output file>

PKG=$1
REPORT_OUTPUT=$2
HTML_REPORT_OUTPUT=$3

COVERAGE_DIR=${COVERAGE_DIR:-coverage}
PKG_LIST=$(go list ${PKG}/... | grep -v /vendor/)

# Create the coverage files directory
mkdir -p ${COVERAGE_DIR};

# Create a coverage file for each package
for package in ${PKG_LIST}; do
    go test -covermode=count -coverprofile "${COVERAGE_DIR}/${package##*/}.cov" ${package} ;
done ;

# Merge the coverage profile files
echo 'mode: count' > "${COVERAGE_DIR}/coverage.cov" ;
tail -q -n +2 "${COVERAGE_DIR}"/*.cov >> "${COVERAGE_DIR}/coverage.cov" ;

# Display the global code coverage
go tool cover -func="${COVERAGE_DIR}/coverage.cov" -o ${REPORT_OUTPUT} ;

# If needed, generate HTML report
if [ -z "$3" ]; then
    go tool cover -html="${COVERAGE_DIR}/coverage.cov" -o ${HTML_REPORT_OUTPUT} ;
fi
