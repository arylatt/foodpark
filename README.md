# foodPark Slack Notifier

Parses the foodPark webpage (https://foodparkcam.com/whos-trading), aggregates the names and links to the venues, and then uses legacy Slack incoming-webhook to send data.

## Basic Usage

1. Set env vars for `FP_SLACK_CHANNEL` and `FP_SLACK_WEBHOOK` as appropriate:
    ``` plaintext
    FP_SLACK_CHANNEL=#general
    FP_SLACK_WEBHOOK=https://hooks.slack.com/services/{blah}/{blah}/{blah}
    ```
1. Run the program