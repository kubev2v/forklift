import os
import re

def update_files_in_folder(folder_path):
    # Regular expressions for each replacement
    patterns = {
        r'"main"': '"release-2.7"',
        r'application: forklift-operator': 'application: forklift-operator-2-7',
        r'component: ([\w-]+)': r'component: \1-2-7',
        r'rh-mtv-1-tenant/forklift-operator/([\w-]+)': r'rh-mtv-1-tenant/forklift-operator-2-7/\1-2-7'
    }

    # Loop through all files in the directory
    for root, _, files in os.walk(folder_path):
        for file_name in files:
            file_path = os.path.join(root, file_name)
            # Read the content of the file
            with open(file_path, 'r', encoding='utf-8') as file:
                content = file.read()

            # Apply all pattern replacements
            for pattern, replacement in patterns.items():
                content = re.sub(pattern, replacement, content)

            # Write the updated content back to the file
            with open(file_path, 'w', encoding='utf-8') as file:
                file.write(content)

            print(f"Updated file: {file_path}")

# Specify the folder path where the files are located
folder_path = '../.tekton'
update_files_in_folder(folder_path)