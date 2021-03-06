/**
 * Given an issue, extracts the start date for the issue.
 */
define(['linq',
        'lib/statusFilter',
        'lib/issueStatusExtractor',
        'lib/issueDateFormatter'],
    function (linq, statusFilter, issueStatusExtractor, issueDateFormatter) {
        'use strict';
        /**
         * Given an issue, extracts the start date for the issue
         * @param  {Objct}      issue The issue
         * @param  {Function}   callback The callback function of the form function (err, startDate)
         */
        return function (issue, callback) {
            issueStatusExtractor(issue, function (err, statuses) {
                if (err) {
                    callback(err);
                }
                else {
                    statusFilter(statuses,
                        function (status) {
                            return status.from === "Open" &&
                                status.to !== "Triage" &&
                                status.to !== "Closed";
                        },
                        function (error, possibleCommitmentPoints) {
                            var startDate;
                            startDate = linq.from(possibleCommitmentPoints)
                                            .select(function (status) {
                                                return status.date;
                                            })
                                            .firstOrDefault();
                            issueDateFormatter(startDate,
                                function (err, formattedStartDate) {
                                    callback(null, formattedStartDate);
                                }
                            );
                        }
                    );
                }
            });
        };
    }
);
