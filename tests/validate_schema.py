# Copyright 2026 XMiete Core Contributors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import json
import os
import sys
from jsonschema import validate, ValidationError

# Files in examples/ that are reference/annotation documents, not schema instances.
# These are not validated against xmiete_schema.json.
NON_SCHEMA_EXAMPLES = {
    "qeaa_deposit_pledge_attestation.json",  # SD-JWT credential format reference
}

def load_schema(schema_path):
    with open(schema_path, 'r') as f:
        return json.load(f)

def get_schema_version(schema):
    return schema.get("properties", {}).get("meta", {}).get("properties", {}).get("version", {}).get("const")

def validate_json(schema, json_path):
    with open(json_path, 'r') as f:
        instance = json.load(f)
    try:
        validate(instance=instance, schema=schema)
        return True, None
    except ValidationError as e:
        return False, e.message

def main():
    schema_path = 'xmiete_schema.json'
    examples_dir = 'examples'
    invalid_dir = 'tests/invalid_examples'

    schema = load_schema(schema_path)
    schema_version = get_schema_version(schema)

    success = True

    print(f"--- Schema version: {schema_version} ---")
    if not schema_version:
        print("❌ meta.version has no 'const' — schema version is not enforced!")
        success = False

    print("\n--- Validating Official Examples ---")
    for filename in sorted(os.listdir(examples_dir)):
        if not filename.endswith('.json'):
            continue
        if filename in NON_SCHEMA_EXAMPLES:
            print(f"⏭  {filename}: Skipped (reference document, not a schema instance)")
            continue
        path = os.path.join(examples_dir, filename)
        valid, error = validate_json(schema, path)
        if valid:
            print(f"✅ {filename}: Valid")
        else:
            print(f"❌ {filename}: Invalid - {error}")
            success = False

    print("\n--- Testing Invalid Examples (Expected to Fail) ---")
    for filename in sorted(os.listdir(invalid_dir)):
        if not filename.endswith('.json'):
            continue
        path = os.path.join(invalid_dir, filename)
        valid, error = validate_json(schema, path)
        if not valid:
            print(f"✅ {filename}: Failed as expected - {error}")
        else:
            print(f"❌ {filename}: Error - This file should have failed validation!")
            success = False

    if not success:
        sys.exit(1)

if __name__ == "__main__":
    main()
