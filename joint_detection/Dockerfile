FROM python:3.8
LABEL maintainer="Patrick Bucher <patrick.bucher@stud.hslu.ch>"
ENV WORKDIR=/usr/src/app
WORKDIR $WORKDIR
COPY requirements.txt $WORKDIR
RUN pip install -r requirements.txt
COPY joint_detection.py $WORKDIR
CMD ["python", "joint_detection.py"]
