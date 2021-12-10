<!--
    Written in the format prescribed by https://github.com/Financial-Times/runbook.md.
    Any future edits should abide by this format.
-->
# UPP - Annotations Monitoring Service

This service is responsible for monitoring annotation publishes.

## Code

annotations-monitoring-service

## Primary URL

https://upp-prod-delivery-glb.upp.ft.com/__annotations-monitoring-service

## Service Tier

Bronze

## Lifecycle Stage

Production

## Host Platform

AWS

## Architecture

The annotations monitoring service is responsible for closing (logging a PublishEnd event for) completed transactions when publishing an annotation.
The basic algorithm is:

```
    every 5 minutes repeat {
            1) call splunk-event-reader to determine the lookbackPeriod (last successful PublishEnd event)
            2) call splunk-event-reader to receive all the open annotations transactions for the lookbackPeriod (transactions with no PublishEnd event)
            3) close completed transactions (valid annotation messages with successful Neo4j write event)
            4) call splunk-event-reader to receive earlier unclosed transactions - check for lookbackPeriod + supersededLookbackPeriod
            5) close events that have been superseded by recent publishes
    }
```

## Contains Personal Data

No

## Contains Sensitive Data

No

<!-- Placeholder - remove HTML comment markers to activate
## Can Download Personal Data
Choose Yes or No

...or delete this placeholder if not applicable to this system
-->

<!-- Placeholder - remove HTML comment markers to activate
## Can Contact Individuals
Choose Yes or No

...or delete this placeholder if not applicable to this system
-->

## Failover Architecture Type

ActiveActive

## Failover Process Type

FullyAutomated

## Failback Process Type

PartiallyAutomated

## Failover Details

See the [failover guide](https://github.com/Financial-Times/upp-docs/tree/master/failover-guides/delivery-cluster) for more details.

## Data Recovery Process Type

NotApplicable

## Data Recovery Details

The service does not store data, so it does not require any data recovery steps.

## Release Process Type

PartiallyAutomated

## Rollback Process Type

Manual

## Release Details

The release is triggered by making a Github release which is then picked up by a Jenkins multibranch pipeline. The Jenkins pipeline should be manually started in order to deploy the helm package to the Kubernetes clusters.

<!-- Placeholder - remove HTML comment markers to activate
## Heroku Pipeline Name
Enter descriptive text satisfying the following:
This is the name of the Heroku pipeline for this system. If you don't have a pipeline, this is the name of the app in Heroku. A pipeline is a group of Heroku apps that share the same codebase where each app in a pipeline represents the different stages in a continuous delivery workflow, i.e. staging, production.

...or delete this placeholder if not applicable to this system
-->

## Key Management Process Type

NotApplicable

## Key Management Details

There is no key rotation procedure for this system.

## Monitoring

Look for the pods in the cluster health endpoint and click to see pod health and checks:

*   <https://upp-prod-delivery-eu.upp.ft.com/__health/__pods-health?service-name=annotations-monitoring-service>
*   <https://upp-prod-delivery-us.upp.ft.com/__health/__pods-health?service-name=annotations-monitoring-service>

## First Line Troubleshooting

<https://github.com/Financial-Times/upp-docs/tree/master/guides/ops/first-line-troubleshooting>

## Second Line Troubleshooting

Please refer to the GitHub repository README for troubleshooting information.