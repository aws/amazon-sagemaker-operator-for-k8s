# `upload_mnist`
This script uploads the MNIST dataset that is compatible with the [Amazon SageMaker XGBoost algorithm](https://docs.aws.amazon.com/sagemaker/latest/dg/xgboost.html) to the specified S3 bucket.
It downloads the MNIST dataset, splits it into train, test, and validation partitions, then uploads the partitions as CSV files into S3.

## Dataset
The dataset is found here: [http://deeplearning.net/data/mnist/mnist.pkl.gz](http://deeplearning.net/data/mnist/mnist.pkl.gz). The code that converts the pickled numpy format to CSV was inspired by the [Amazon SageMaker Developer Guide](https://docs.aws.amazon.com/sagemaker/latest/dg/sagemaker-dg.pdf).

## Example:

```bash
./upload_xgboost_mnist_dataset --s3-bucket ${BUCKET_NAME} --s3-prefix mnist-data
```

## Requirements
* `Python3`
* `boto3`
* `numpy`
* `argparse`
