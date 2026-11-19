.. Comments are hidden, like the licence headers Sphinx projects start with.

.. |rexmd| replace:: **rexmd**

.. _top:

A tour of reStructuredText
==========================

|rexmd| renders reStructuredText too, including the Sphinx dialect: *emphasis*,
**strong**, ``literals``, `links <https://www.sphinx-doc.org/>`_, footnotes
[#]_, and roles such as :class:`~airflow.models.dag.DAG` and :func:`json.dumps`.

.. tip:: Press ``t`` for a table of contents and ``?`` for every key.

.. contents::
   :local:

Admonitions
-----------

.. note::

   Admonitions share the colours of GitHub's alerts.

.. warning::

   Mermaid diagrams need ``rexmd setup --mermaid``.

.. versionadded:: 0.2
   Support for reStructuredText.

Code
----

.. code-block:: python

   def greet(name: str) -> str:
       return f"hello, {name}"

A paragraph ending in a double colon starts a literal block::

   rexmd examples/tour.rst

>>> print("doctest blocks keep their prompts")
doctest blocks keep their prompts

Math and images
---------------

Inline math like :math:`e^{i\pi} + 1 = 0` sits on the line.

.. math::

   \int_0^\infty e^{-x^2}\,dx = \frac{\sqrt{\pi}}{2}

.. figure:: assets/landscape.png
   :width: 320px

   A figure's caption sits beneath it.

Tables
------

+---------------+-------------+
| Construct     | Rendering   |
+===============+=============+
| Grid tables   | Box drawing |
+---------------+-------------+
| Simple tables | The same    |
+---------------+-------------+

=============  ===========
Simple tables  look alike
=============  ===========
Header row     optional
=============  ===========

.. list-table::
   :header-rows: 1

   * - Directive
     - Shown as
   * - ``tab-set``
     - Stacked panels
   * - Unknown
     - A muted panel

Lists
-----

#. Enumerated lists
#. With nesting

   * bullets change by depth

Term
   Definition lists indent the definition.

.. tab-set::

   .. tab-item:: uv

      Tab items become titled panels.

   .. tab-item:: pip

      One after another.

Unknown directives
------------------

Directives from Sphinx extensions rexmd doesn't know show their name,
arguments and options:

.. exampleinclude:: /example_dags/example_bash.py
   :language: python
   :start-after: [START howto_operator_bash]

See `Admonitions`_ again, or jump to :ref:`the top <top>`.

.. [#] Footnotes collect where they are defined.
