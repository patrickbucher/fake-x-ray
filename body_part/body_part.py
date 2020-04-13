#!/usr/bin/env python3

import pika


def main():
    connection = pika.BlockingConnection(
        pika.ConnectionParameters('localhost'))
    channel = connection.channel()
    channel.queue_declare(queue='xrays')
    channel.basic_consume(
        queue='xrays',
        auto_ack=True,
        on_message_callback=callback)
    channel.start_consuming()


def callback(ch, method, properties, body):
    print(f'received {body} from channel')


if __name__ == '__main__':
    main()
