#!/bin/sh
node bin/extractFields.js -e lib/issueNameExtractor.js,name -e lib/issueCreatedDateExtractor.js,createdDate -e lib/issueStatusExtractor.js,statuses $1
