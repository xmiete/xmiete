import json
import os
import sys
from jsonschema import validate, ValidationError

def validate_json(schema_path, json_path):
    with open(schema_path, 'r') as f:
        schema = json.load(f)
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
    
    success = True

    print("--- Validating Official Examples ---")
    for filename in os.listdir(examples_dir):
        if filename.endswith('.json'):
            path = os.path.join(examples_dir, filename)
            valid, error = validate_json(schema_path, path)
            if valid:
                print(f"✅ {filename}: Valid")
            else:
                print(f"❌ {filename}: Invalid - {error}")
                success = False

    print("\n--- Testing Invalid Examples (Expected to Fail) ---")
    for filename in os.listdir(invalid_dir):
        if filename.endswith('.json'):
            path = os.path.join(invalid_dir, filename)
            valid, error = validate_json(schema_path, path)
            if not valid:
                print(f"✅ {filename}: Failed as expected - {error}")
            else:
                print(f"❌ {filename}: Error - This file should have failed validation!")
                success = False

    if not success:
        sys.exit(1)

if __name__ == "__main__":
    main()
