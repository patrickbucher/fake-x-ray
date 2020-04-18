#!/usr/bin/env python3

import json
import os
import sys
import time

import pika


def main():
    amqp_uri = os.environ.get('AMQP_URI', 'localhost')
    max_retries = 10
    retry_after_seconds = 2
    retries = 0
    connection = None
    while retries < max_retries:
        try:
            connection = pika.BlockingConnection(
                pika.ConnectionParameters(amqp_uri))
            break
        except pika.exceptions.AMQPConnectionError as e:
            print(e)
            retries += 1
            print(f'connection to {amqp_uri} failed, waiting...')
            time.sleep(retry_after_seconds)

    if not connection:
        print(f'unable to connect to {amqp_uri} after {retries} retries')
        sys.exit(1)
    print(f'connected to to {amqp_uri}')

    channel = connection.channel()
    channel.queue_declare(queue='joints')
    channel.basic_qos(prefetch_count=1)
    channel.basic_consume(
        queue='joints',
        on_message_callback=callback)
    channel.start_consuming()


def callback(ch, method, props, body):
    print('task', body)
    joint, score = score_joint(body.decode('utf-8'))
    print('joint', joint, 'score', score)
    response = json.dumps({'joint': joint, 'score': score})
    ch.basic_publish(
        exchange='',
        routing_key='scores',
        properties=pika.BasicProperties(
            correlation_id=props.correlation_id),
        body=response)
    ch.basic_ack(delivery_tag=method.delivery_tag)


def score_joint(body):
    parts = body.split('=')
    if len(parts) != 2:
        return ''
    joint = str(parts[0])
    score = int(parts[1])
    return joint, score


if __name__ == '__main__':
    main()
