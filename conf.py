# Configuration file for the Sphinx documentation builder.
#
# This file only contains a selection of the most common options. For a full
# list see the documentation:
# https://www.sphinx-doc.org/en/master/usage/configuration.html

# -- Path setup --------------------------------------------------------------

# If extensions (or modules to document with autodoc) are in another directory,
# add these directories to sys.path here. If the directory is relative to the
# documentation root, use os.path.abspath to make it absolute, like shown here.
#
# import os
# import sys
# sys.path.insert(0, os.path.abspath('.'))
from docutils import nodes
from os.path import isdir, isfile, join, basename, dirname
from os import makedirs, getenv
from shutil import copyfile
from pygments.lexers.go import GoLexer
from sphinx.highlighting import lexers

#############
#
# Add a special lexer to add a class to console lexer
#
#############

class goLangLexer (GoLexer):
    name = 'golang'

lexers['golang'] = goLangLexer(startinLine=True)

# -- Project information -----------------------------------------------------

project = 'Intel® Device Plugins for Kubernetes'
copyright = '2020, various'
author = 'various'


##############################################################################
#
# This section determines the behavior of links to local items in .md files.
#
#  if useGitHubURL == True:
#
#     links to local files and directories will be turned into github URLs
#     using either the baseBranch defined here or using the commit SHA.
#
#  if useGitHubURL == False:
#
#     local files will be moved to the website directory structure when built
#     local directories will still be links to github URLs
#
#  if built with GitHub workflows:
#
#     the GitHub URLs will use the commit SHA (GITHUB_SHA environment variable
#     is defined by GitHub workflows) to link to the specific commit.
#
##############################################################################

baseBranch = "main"
sphinx_md_useGitHubURL = True
commitSHA = getenv('GITHUB_SHA')
githubBaseURL = 'https://github.com/' + (getenv('GITHUB_REPOSITORY') or 'intel/intel-device-plugins-for-kubernetes') + '/'
githubFileURL = githubBaseURL + "blob/"
githubDirURL = githubBaseURL + "tree/"
if commitSHA:
    githubFileURL = githubFileURL + commitSHA + "/"
    githubDirURL = githubDirURL + commitSHA + "/"
else:
    githubFileURL = githubFileURL + baseBranch + "/"
    githubDirURL = githubDirURL + baseBranch + "/"
sphinx_md_githubFileURL = githubFileURL
sphinx_md_githubDirURL = githubDirURL

# -- General configuration ---------------------------------------------------

# Add any Sphinx extension module names here, as strings. They can be
# extensions coming with Sphinx (named 'sphinx.ext.*') or your custom
# ones.
extensions = ['recommonmark','sphinx_markdown_tables','sphinx_md']
source_suffix = {'.rst': 'restructuredtext','.md': 'markdown'}


# Add any paths that contain templates here, relative to this directory.
templates_path = ['_templates']

# List of patterns, relative to source directory, that match files and
# directories to ignore when looking for source files.
# This pattern also affects html_static_path and html_extra_path.
exclude_patterns = ['_build', 'Thumbs.db', '.DS_Store','_work','cmd/fpga_plugin/pictures']


# -- Options for HTML output -------------------------------------------------

# The theme to use for HTML and HTML Help pages.  See the documentation for
# a list of builtin themes.
#
html_theme = 'sphinx_rtd_theme'

# Add any paths that contain custom static files (such as style sheets) here,
# relative to this directory. They are copied after the builtin static files,
# so a file named "default.css" will overwrite the builtin "default.css".
#html_static_path = ['_static']
