# This function replaces values in testfile in place

import fileinput
import argparse
import sys
from sagemaker import image_uris


def get_parser():
    parser = argparse.ArgumentParser(description='Replace testfile values for k8s SM operators')
    parser.add_argument('--testfile_path', type=str, required=True, nargs='+', help='Path to test files which will be edited in place')
    parser.add_argument('--sagemaker_region', type=str, required=True, help='Sagemaker region')
    parser.add_argument('--sagemaker_role', type=str, required=True, help='Sagemaker executor role ARN')
    parser.add_argument('--bucket_name', type=str, required=True, help='S3 data bucket name')
    return parser

def replace_in_file(file_path, replace_values):
    print("Replacing values in file " + file_path)
    # Everything printed to stdout will be stored to file
    with fileinput.FileInput(file_path, inplace=True) as file:
        for line in file:
            for find_value, replace_value in replace_values:
                line = line.replace(find_value, replace_value)
            print(line, end='')

def main(argv):
    parser = get_parser()
    args = parser.parse_args(argv)
    replace_values = [
        ('{ROLE_ARN}', args.sagemaker_role),
        ('{REGION}', args.sagemaker_region),
        ('{DATA_BUCKET}', args.bucket_name),
        ('{XGBOOST_IMAGE}', image_uris.retrieve(framework='xgboost', region=args.sagemaker_region, version='1')),
        ('{SAGEMAKER_XGBOOST_IMAGE}', image_uris.retrieve(framework='xgboost', region=args.sagemaker_region, version='0.90-2')),
        ('{SAGEMAKER_DEBUGGER_RULES_IMAGE}', image_uris.retrieve(framework='debugger', region=args.sagemaker_region, version='latest')),
        ('{FAKE_IMAGE}', 'ubuntu:latest')
    ]
    for file in args.testfile_path:
        replace_in_file(file, replace_values)

if __name__ == "__main__":
    main(sys.argv[1:])