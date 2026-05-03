#!/bin/bash

# Create the Serbian directory if it doesn't exist
mkdir -p sr

echo "🇷🇸 Scaffolding Serbian documentation..."

# Loop through all HTML files in the current directory
for file in *.html; do
    if [ -f "$file" ]; then
        cp "$file" "sr/$file"
        echo "  → Copied $file to sr/$file"
        
        # Optional: If you use relative paths for your CSS (like href="style.css"),
        # you need them to point up one directory (href="../style.css") in the /sr folder.
        # Uncomment the lines below to auto-fix standard relative CSS links (works on macOS/Linux).
        
        # if [[ "$OSTYPE" == "darwin"* ]]; then
        #     sed -i '' 's/href="style.css"/href="..\/style.css"/g' "sr/$file"
        # else
        #     sed -i 's/href="style.css"/href="..\/style.css"/g' "sr/$file"
        # fi
    fi
done

echo ""
echo "✅ Done! Now you can start translating the files in the /sr/ directory."
echo "⚠️  Remember: If your images or CSS use relative paths, update them in the /sr/ HTML files to point to '../'"
