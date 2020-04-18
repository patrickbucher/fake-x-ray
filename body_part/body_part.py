#!/usr/bin/env python3

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
    channel.queue_declare(queue='xrays')
    channel.queue_declare(queue='body_part')
    channel.basic_qos(prefetch_count=1)
    channel.basic_consume(queue='xrays', on_message_callback=on_request)
    channel.start_consuming()


def on_request(ch, method, props, body):
    print('request', body)
    response = detect_body_part(body.decode('utf-8'))
    ch.basic_publish(
        exchange='',
        routing_key='body_part',
        properties=pika.BasicProperties(correlation_id=props.correlation_id),
        body=response)
    print('response', response)
    ch.basic_ack(delivery_tag=method.delivery_tag)


def detect_body_part(body):
    parts = body.split('(')
    detected_part = ''
    if len(parts) > 0:
        detected_part = parts[0]
    return detected_part


if __name__ == '__main__':
    main()
