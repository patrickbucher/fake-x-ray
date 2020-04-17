#!/usr/bin/env python3

import pika


def main():
    connection = pika.BlockingConnection(
        pika.ConnectionParameters('localhost'))
    channel = connection.channel()
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
    print('ack')


def detect_body_part(body):
    parts = body.split('(')
    detected_part = ''
    if len(parts) > 0:
        detected_part = parts[0]
    return detected_part


if __name__ == '__main__':
    main()
