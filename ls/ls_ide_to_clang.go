package ls

import (
	"fmt"

	"github.com/arduino/arduino-language-server/sourcemapper"
	"go.bug.st/lsp"
	"go.bug.st/lsp/jsonrpc"
)

func (ls *INOLanguageServer) idePathToIdeURI(logger jsonrpc.FunctionLogger, inoPath string) (lsp.DocumentURI, error) {
	if inoPath == sourcemapper.NotIno.File {
		return sourcemapper.NotInoURI, nil
	}
	doc, ok := ls.trackedIdeDocs[inoPath]
	if !ok {
		logger.Logf("    !!! Unresolved .ino path: %s", inoPath)
		logger.Logf("    !!! Known doc paths are:")
		for p := range ls.trackedIdeDocs {
			logger.Logf("    !!! > %s", p)
		}
		uri := lsp.NewDocumentURI(inoPath)
		return uri, &UnknownURI{uri}
	}
	return doc.URI, nil
}

func (ls *INOLanguageServer) ide2ClangTextDocumentIdentifier(logger jsonrpc.FunctionLogger, ideTextDocIdentifier lsp.TextDocumentIdentifier) (lsp.TextDocumentIdentifier, error) {
	clangURI, _, err := ls.ide2ClangDocumentURI(logger, ideTextDocIdentifier.URI)
	return lsp.TextDocumentIdentifier{URI: clangURI}, err
}

func (ls *INOLanguageServer) ide2ClangDocumentURI(logger jsonrpc.FunctionLogger, ideURI lsp.DocumentURI) (lsp.DocumentURI, bool, error) {
	// Sketchbook/Sketch/Sketch.ino      -> build-path/sketch/Sketch.ino.cpp
	// Sketchbook/Sketch/AnotherTab.ino  -> build-path/sketch/Sketch.ino.cpp  (different section from above)
	idePath := ideURI.AsPath()
	if idePath.Ext() == ".ino" {
		clangURI := lsp.NewDocumentURIFromPath(ls.buildSketchCpp)
		logger.Logf("URI: %s -> %s", ideURI, clangURI)
		return clangURI, true, nil
	}

	// another/path/source.cpp -> another/path/source.cpp (unchanged)
	inside, err := idePath.IsInsideDir(ls.sketchRoot)
	if err != nil {
		logger.Logf("ERROR: could not determine if '%s' is inside '%s'", idePath, ls.sketchRoot)
		return lsp.NilURI, false, &UnknownURI{ideURI}
	}
	if !inside {
		clangURI := ideURI
		logger.Logf("URI: %s -> %s", ideURI, clangURI)
		return clangURI, false, nil
	}

	// Sketchbook/Sketch/AnotherFile.cpp -> build-path/sketch/AnotherFile.cpp
	rel, err := ls.sketchRoot.RelTo(idePath)
	if err != nil {
		logger.Logf("ERROR: could not determine rel-path of '%s' in '%s': %s", idePath, ls.sketchRoot, err)
		return lsp.NilURI, false, err
	}

	clangPath := ls.buildSketchRoot.JoinPath(rel)
	clangURI := lsp.NewDocumentURIFromPath(clangPath)
	logger.Logf("URI: %s -> %s", ideURI, clangURI)
	return clangURI, true, nil
}

func (ls *INOLanguageServer) ide2ClangTextDocumentPositionParams(logger jsonrpc.FunctionLogger, ideParams lsp.TextDocumentPositionParams) (lsp.TextDocumentPositionParams, error) {
	clangURI, clangPosition, err := ls.ide2ClangPosition(logger, ideParams.TextDocument.URI, ideParams.Position)
	if err != nil {
		logger.Logf("Error converting position %s: %s", ideParams, err)
		return lsp.TextDocumentPositionParams{}, err
	}
	clangParams := lsp.TextDocumentPositionParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: clangURI},
		Position:     clangPosition,
	}
	logger.Logf("%s -> %s", ideParams, clangParams)
	return clangParams, nil
}

func (ls *INOLanguageServer) ide2ClangPosition(logger jsonrpc.FunctionLogger, ideURI lsp.DocumentURI, idePosition lsp.Position) (lsp.DocumentURI, lsp.Position, error) {
	clangURI, clangRange, err := ls.ide2ClangRange(logger, ideURI, lsp.Range{Start: idePosition, End: idePosition})
	return clangURI, clangRange.Start, err
}

func (ls *INOLanguageServer) ide2ClangRange(logger jsonrpc.FunctionLogger, ideURI lsp.DocumentURI, ideRange lsp.Range) (lsp.DocumentURI, lsp.Range, error) {
	clangURI, inSketch, err := ls.ide2ClangDocumentURI(logger, ideURI)
	if err != nil {
		return lsp.DocumentURI{}, lsp.Range{}, err
	}

	// Convert .ino ranges using sketchmapper
	if ls.clangURIRefersToIno(clangURI) {
		if clangRange, ok := ls.sketchMapper.InoToCppLSPRangeOk(ideURI, ideRange); ok {
			return clangURI, clangRange, nil
		} else {
			return lsp.DocumentURI{}, lsp.Range{}, fmt.Errorf("invalid range %s:%s: could not be mapped to Arduino-preprocessed sketck.ino.cpp", ideURI, ideRange)
		}
	} else if inSketch {
		// Convert other sketch file ranges (.cpp/.h)
		clangRange := ideRange
		clangRange.Start.Line++
		clangRange.End.Line++
		return clangURI, clangRange, nil
	} else {
		// Outside sketch: keep range as is
		clangRange := ideRange
		return clangURI, clangRange, nil
	}
}

func (ls *INOLanguageServer) ide2ClangVersionedTextDocumentIdentifier(logger jsonrpc.FunctionLogger, ideVersionedDoc lsp.VersionedTextDocumentIdentifier) (lsp.VersionedTextDocumentIdentifier, error) {
	clangURI, _, err := ls.ide2ClangDocumentURI(logger, ideVersionedDoc.URI)
	return lsp.VersionedTextDocumentIdentifier{
		TextDocumentIdentifier: lsp.TextDocumentIdentifier{URI: clangURI},
		Version:                ideVersionedDoc.Version,
	}, err
}
