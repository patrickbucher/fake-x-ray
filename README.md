# fake-x-ray

Mock implementation for my bachelor thesis DeepXRay, using a simple text representation of the desired output instead of images.

## Components

- `orchestrator`: offers an HTTP API to coordinate the three following machine learning models.
- `body_part`: detects whether or not the payload given denotes a left hand.
- `joint_detection`: extracts the joints on the given payload.
- `ratingen_score`: extracts the score from the individual joints.

## Fake Textual Payload

Instead of using actual x-ray images, the following textual representation denotes a left hands with the ten joints of interest:

    HAND_LEFT(mcp1=30, mcp2=0, mcp3=15, mcp4=10, mcp5=0, pip1=35, pip2=45, pip3=70, pip4=50, pip5=30)

Which should yield the following result:

```json
{
  "mcp1": 30,
  "mcp2": 0,
  "mcp3": 15,
  "mcp4": 10,
  "mcp5": 0,
  "pip1": 35,
  "pip2": 45,
  "pip3": 70,
  "pip4": 50,
  "pip5": 30
}
```

## How To

### Setup

On Arch Linux:

```bash
$ sudo pacman -S rabbitmq rabbitmqadmin
$ sudo systemctl start rabbitmq
$ rabbitmq-plugins enable rabbitmq_management
```

Open [Admin UI](http://localhost:15672/) with username `guest` and password `guest`.

### Example

```bash
$ curl -X POST localhost:8080/score -d @example.json
```
