from setuptools import setup, find_packages

setup(
    name="bound-ai",
    version="0.1.0",
    description="Programmable spend vaults and financial rails for autonomous AI agents.",
    long_description=open("README.md", encoding="utf-8").read(),
    long_description_content_type="text/markdown",
    author="404kernel",
    url="https://github.com/404kernel/bound",
    packages=find_packages(),
    python_requires=">=3.8",
    classifiers=[
        "Programming Language :: Python :: 3",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
        "Topic :: Security",
        "Topic :: Software Development :: Libraries",
    ],
)
