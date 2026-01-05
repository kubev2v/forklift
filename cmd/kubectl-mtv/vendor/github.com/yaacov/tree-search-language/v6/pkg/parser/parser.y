%{
package parser

// Global variables for the parser
var parseResult *Node
var parseError error
var currentLexer *Lexer
%}

// Union type for semantic values
%union {
    node   *Node
    str    string
    pos    int
}

// Token declarations
%token K_LIKE K_ILIKE K_AND K_OR K_BETWEEN K_IN K_IS K_NULL
%token K_NOT K_TRUE K_FALSE K_LEN K_ANY K_ALL K_SUM
%token <str> NUMERIC_LITERAL STRING_LITERAL IDENTIFIER DATE RFC3339
%token LPAREN RPAREN COMMA
%token PLUS MINUS STAR SLASH PERCENT
%token LBRACKET RBRACKET
%token EQ NE LT LE GT GE REQ RNE
%token UMINUS

// Operator precedence and associativity (lowest to highest)
%left K_OR                         
%left K_AND
%left EQ NE LT LE GT GE REQ RNE
%left K_LIKE K_ILIKE K_IS K_BETWEEN K_IN
%left PLUS MINUS                   
%left STAR SLASH PERCENT           
%right K_NOT K_LEN K_ANY K_ALL K_SUM   
%right UMINUS                      

// Non-terminal types
%type <node> input expr or_expr and_expr comparison_expr
%type <node> additive_expr multiplicative_expr not_expr unary_expr
%type <node> primary array array_elements opt_array_elements

// Start symbol
%start input

%%

input:
    expr { parseResult = $1 }
    ;

expr:
    or_expr
    ;

or_expr:
      and_expr
    | or_expr K_OR and_expr        { $$ = NewBinaryOpNode(OpOr, $1, $3, 0) }
    ;

and_expr:
      comparison_expr
    | and_expr K_AND comparison_expr      { $$ = NewBinaryOpNode(OpAnd, $1, $3, 0) }
    ;

comparison_expr:
      additive_expr
    | comparison_expr EQ additive_expr      { $$ = NewBinaryOpNode(OpEQ, $1, $3, 0) }
    | comparison_expr NE additive_expr      { $$ = NewBinaryOpNode(OpNE, $1, $3, 0) }
    | comparison_expr LT additive_expr      { $$ = NewBinaryOpNode(OpLT, $1, $3, 0) }
    | comparison_expr LE additive_expr      { $$ = NewBinaryOpNode(OpLE, $1, $3, 0) }
    | comparison_expr GT additive_expr      { $$ = NewBinaryOpNode(OpGT, $1, $3, 0) }
    | comparison_expr GE additive_expr      { $$ = NewBinaryOpNode(OpGE, $1, $3, 0) }
    | comparison_expr REQ additive_expr     { $$ = NewBinaryOpNode(OpREQ, $1, $3, 0) }
    | comparison_expr RNE additive_expr     { $$ = NewBinaryOpNode(OpRNE, $1, $3, 0) }
    | comparison_expr K_LIKE additive_expr  { $$ = NewBinaryOpNode(OpLike, $1, $3, 0) }
    | comparison_expr K_ILIKE additive_expr { $$ = NewBinaryOpNode(OpILike, $1, $3, 0) }
    | comparison_expr K_NOT K_LIKE additive_expr  {
        likeExpr := NewBinaryOpNode(OpLike, $1, $4, 0)
        $$ = NewUnaryOpNode(OpNot, likeExpr, 0)
    }
    | comparison_expr K_NOT K_ILIKE additive_expr {
        ilikeExpr := NewBinaryOpNode(OpILike, $1, $4, 0)
        $$ = NewUnaryOpNode(OpNot, ilikeExpr, 0)
    }
    | comparison_expr K_IS K_NULL           { $$ = NewBinaryOpNode(OpIs, $1, NewNullNode(0), 0) }
    | comparison_expr K_IS K_NOT K_NULL     {
        isNullExpr := NewBinaryOpNode(OpIs, $1, NewNullNode(0), 0)
        $$ = NewUnaryOpNode(OpNot, isNullExpr, 0)
    }
    | comparison_expr K_BETWEEN additive_expr K_AND additive_expr {
        rangeArray := NewArrayNode([]*Node{$3, $5}, 0)
        $$ = NewBinaryOpNode(OpBetween, $1, rangeArray, 0)
    }
    | comparison_expr K_NOT K_BETWEEN additive_expr K_AND additive_expr {
        rangeArray := NewArrayNode([]*Node{$4, $6}, 0)
        betweenExpr := NewBinaryOpNode(OpBetween, $1, rangeArray, 0)
        $$ = NewUnaryOpNode(OpNot, betweenExpr, 0)
    }
    | comparison_expr K_IN additive_expr     { $$ = NewBinaryOpNode(OpIn, $1, $3, 0) }
    | comparison_expr K_NOT K_IN additive_expr {
        inExpr := NewBinaryOpNode(OpIn, $1, $4, 0)
        $$ = NewUnaryOpNode(OpNot, inExpr, 0)
    }
    ;

additive_expr:
      multiplicative_expr
    | additive_expr PLUS multiplicative_expr   { $$ = NewBinaryOpNode(OpPlus, $1, $3, 0) }
    | additive_expr MINUS multiplicative_expr  { $$ = NewBinaryOpNode(OpMinus, $1, $3, 0) }
    ;

multiplicative_expr:
      not_expr
    | multiplicative_expr STAR not_expr    { $$ = NewBinaryOpNode(OpStar, $1, $3, 0) }
    | multiplicative_expr SLASH not_expr   { $$ = NewBinaryOpNode(OpSlash, $1, $3, 0) }
    | multiplicative_expr PERCENT not_expr { $$ = NewBinaryOpNode(OpPercent, $1, $3, 0) }
    ;

not_expr:
      unary_expr
    | K_NOT not_expr               { $$ = NewUnaryOpNode(OpNot, $2, 0) }
    | K_LEN not_expr               { $$ = NewUnaryOpNode(OpLen, $2, 0) }
    | K_ANY not_expr               { $$ = NewUnaryOpNode(OpAny, $2, 0) }
    | K_ALL not_expr               { $$ = NewUnaryOpNode(OpAll, $2, 0) }
    | K_SUM not_expr               { $$ = NewUnaryOpNode(OpSum, $2, 0) }
    ;

unary_expr:
      primary
    | MINUS unary_expr             { $$ = NewUnaryOpNode(OpUMinus, $2, 0) }
    | PLUS unary_expr              { $$ = $2 }  // unary plus is a no-op
    | LPAREN expr RPAREN           { $$ = $2 }
    | array                        { $$ = $1 }
    ;

array:
      LBRACKET opt_array_elements RBRACKET { $$ = $2 }
    ;

opt_array_elements:
      /* empty */                 { $$ = NewArrayNode([]*Node{}, 0) }
    | array_elements              { $$ = $1 }
    | array_elements COMMA        { $$ = $1 } // trailing comma
    ;

array_elements:
      expr                          {
                                      $$ = NewArrayNode([]*Node{$1}, 0)
                                    }
    | array_elements COMMA expr     {
                                      // Append to existing array
                                      $1.Children = append($1.Children, $3)
                                      $$ = $1
                                    }
    ;

primary:
      NUMERIC_LITERAL       { $$ = NewNumberNode($1, 0) }
    | STRING_LITERAL        { $$ = NewStringNode($1, 0) }
    | IDENTIFIER            { $$ = NewIdentifierNode($1, 0) }
    | RFC3339               { $$ = NewTimestampNode($1, 0) }
    | DATE                  { $$ = NewDateNode($1, 0) }
    | K_TRUE                { $$ = NewBooleanNode(true, 0) }
    | K_FALSE               { $$ = NewBooleanNode(false, 0) }
    ;

%%
