import io
import re
from typing import List, Tuple, Optional
from pathlib import Path
import fitz  # PyMuPDF
import markdown
from bs4 import BeautifulSoup
from loguru import logger


class DocumentParser:
    @staticmethod
    async def parse_pdf(file_content: bytes) -> Tuple[str, List[int]]:
        """
        Parse PDF content and return text with page numbers.
        Returns: (full_text, list_of_page_numbers)
        """
        try:
            pdf_document = fitz.open(stream=file_content, filetype="pdf")
            full_text = ""
            page_numbers = []
            
            for page_num in range(pdf_document.page_count):
                page = pdf_document[page_num]
                page_text = page.get_text()
                
                if page_text.strip():
                    full_text += f"\n[Page {page_num + 1}]\n{page_text}"
                    page_numbers.append(page_num + 1)
            
            pdf_document.close()
            return full_text.strip(), page_numbers
        except Exception as e:
            logger.error(f"Error parsing PDF: {e}")
            raise ValueError(f"Failed to parse PDF: {str(e)}")
    
    @staticmethod
    async def parse_markdown(file_content: bytes) -> str:
        """
        Parse Markdown content and convert to plain text while preserving structure.
        """
        try:
            md_text = file_content.decode('utf-8')
            
            # Convert markdown to HTML first
            html = markdown.markdown(md_text, extensions=['tables', 'fenced_code'])
            
            # Parse HTML and extract text
            soup = BeautifulSoup(html, 'html.parser')
            
            # Process different elements to maintain structure
            for tag in soup.find_all(['h1', 'h2', 'h3', 'h4', 'h5', 'h6']):
                tag.string = f"\n{'#' * int(tag.name[1])} {tag.get_text()}\n"
            
            for tag in soup.find_all('p'):
                tag.string = f"{tag.get_text()}\n"
            
            for tag in soup.find_all('li'):
                tag.string = f"• {tag.get_text()}\n"
            
            for tag in soup.find_all('code'):
                tag.string = f"`{tag.get_text()}`"
            
            for tag in soup.find_all('pre'):
                tag.string = f"\n```\n{tag.get_text()}\n```\n"
            
            # Get final text
            text = soup.get_text()
            
            # Clean up excessive newlines
            text = re.sub(r'\n{3,}', '\n\n', text)
            
            return text.strip()
        except Exception as e:
            logger.error(f"Error parsing Markdown: {e}")
            raise ValueError(f"Failed to parse Markdown: {str(e)}")
    
    @staticmethod
    def clean_text(text: str) -> str:
        """
        Clean and normalize text for better chunking.
        """
        # Remove excessive whitespace
        text = re.sub(r'\s+', ' ', text)
        
        # Normalize line breaks
        text = re.sub(r'\n\s*\n', '\n\n', text)
        
        # Remove special characters that might interfere with chunking
        text = re.sub(r'[\x00-\x08\x0B-\x0C\x0E-\x1F\x7F]', '', text)
        
        return text.strip()